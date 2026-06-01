package notifications

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/structify"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1history "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// fetchableEvent wraps both event.Event and relations, allowing to enrich the Event based on Notifications feedback.
type fetchableEvent struct {
	*event.Event
	*relations
}

// completeAndUpdate completes the internal relations and the Event.
//
// This method can be called with a nil slice to populate the event.Event without any fetching.
func (ev *fetchableEvent) completeAndUpdate(ctx context.Context, attributes []string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, attribute := range attributes {
		err := ev.relations.complete(ctx, attribute)
		if err != nil {
			return errors.Wrapf(err, "cannot complete relations for attribute %q", attribute)
		}
	}

	// TODO: consider filtering fetched customvars to requested ones

	ev.Event.Relations = ev.relations.asMap()
	ev.Event.CompleteRelations = ev.relations.completeRelations

	return nil
}

// Client is an Icinga Notifications compatible client implementation to push events to Icinga Notifications.
//
// A new Client should be created by the NewNotificationsClient function. New history entries can be submitted by
// calling the Client.Submit method.
type Client struct {
	source.Config

	db     *database.DB
	logger *logging.Logger

	ctx context.Context

	notificationsClient *source.Client // The Icinga Notifications client used to interact with the API.
	redisClient         *redis.Client  // redisClient is the Redis client used to fetch host and service names for events.
}

// NewNotificationsClient creates a new Client connected to an existing database and logger.
func NewNotificationsClient(
	ctx context.Context,
	db *database.DB,
	rc *redis.Client,
	logger *logging.Logger,
	cfg source.Config,
) (*Client, error) {
	notificationsClient, err := source.NewClient(cfg, "Icinga DB "+internal.Version.Version)
	if err != nil {
		return nil, err
	}

	return &Client{
		Config: cfg,

		db:     db,
		logger: logger,

		ctx: ctx,

		notificationsClient: notificationsClient,
		redisClient:         rc,
	}, nil
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service IDs.
//
// This function is used by all event builders to create a common event structure that includes the host and service
// names, an Icinga DB Web reference, and the tags for the event.
// Any event type-specific information (like severity, message, etc.) is added by the specific event builders.
func (client *Client) buildCommonEvent(
	ctx context.Context,
	hostId, serviceId types.Binary,
) (*fetchableEvent, error) {
	rel, err := client.fetchHostServiceData(ctx, hostId, serviceId)
	if err != nil {
		return nil, err
	}

	var (
		objectName  string
		hostName    string
		serviceName string

		objectUrl  string
		objectTags map[string]string
	)

	if rel.Host == nil {
		return nil, errors.New("relations does not contain a host")
	}
	hostName = rel.Host.Name

	if serviceId != nil {
		if len(rel.Services) == 0 {
			return nil, errors.New("relations does not contain a service")
		}
		serviceName = rel.Services[0].Name

		objectName = hostName + "!" + serviceName
		objectUrl = "/icingadb/service?name=" + utils.RawUrlEncode(serviceName) +
			"&host.name=" + utils.RawUrlEncode(hostName)
		objectTags = map[string]string{
			"host":    hostName,
			"service": serviceName,
		}
	} else {
		objectName = hostName
		objectUrl = "/icingadb/host?name=" + utils.RawUrlEncode(hostName)
		objectTags = map[string]string{
			"host": hostName,
		}
	}

	ev := &fetchableEvent{
		Event: &event.Event{
			Name: objectName,
			URL:  objectUrl,
			Tags: objectTags,
		},
		relations: rel,
	}

	if err = ev.completeAndUpdate(ctx, client.DefaultRelations); err != nil {
		return nil, errors.Wrap(err, "cannot complete event relations")
	}

	return ev, nil
}

// buildStateHistoryEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (client *Client) buildStateHistoryEvent(ctx context.Context, h *v1history.StateHistory) (*fetchableEvent, error) {
	ev, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
	}

	ev.Type = event.TypeState

	if h.ServiceId != nil {
		switch h.HardState {
		case 0:
			ev.Severity = event.SeverityOK
		case 1:
			ev.Severity = event.SeverityWarning
		case 2:
			ev.Severity = event.SeverityCrit
		case 3:
			ev.Severity = event.SeverityErr
		default:
			return nil, fmt.Errorf("unexpected service state %d", h.HardState)
		}
	} else {
		switch h.HardState {
		case 0:
			ev.Severity = event.SeverityOK
		case 1:
			ev.Severity = event.SeverityCrit
		default:
			return nil, fmt.Errorf("unexpected host state %d", h.HardState)
		}
	}

	if h.Output.Valid {
		ev.Message = h.Output.String
	}
	if h.LongOutput.Valid {
		ev.Message += "\n" + h.LongOutput.String
	}

	return ev, nil
}

// buildDowntimeHistoryMetaEvent from a downtime history entry.
func (client *Client) buildDowntimeHistoryMetaEvent(ctx context.Context, h *v1history.DowntimeHistoryMeta) (*fetchableEvent, error) {
	ev, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
	}

	switch h.EventType {
	case "downtime_start":
		ev.Type = event.TypeDowntimeStart
		ev.Username = h.Author
		ev.Message = h.Comment
		ev.Mute = types.MakeBool(true)
		ev.MuteReason = "Checkable is in downtime"

	case "downtime_end":
		ev.Mute = types.MakeBool(false)
		if h.HasBeenCancelled.Valid && h.HasBeenCancelled.Bool {
			ev.Type = event.TypeDowntimeRemoved
			ev.Message = "Downtime was cancelled"

			if h.CancelledBy.Valid {
				ev.Username = h.CancelledBy.String
			}
		} else {
			ev.Type = event.TypeDowntimeEnd
			ev.Message = "Downtime expired"
		}

	default:
		return nil, fmt.Errorf("unexpected event type %q", h.EventType)
	}

	return ev, nil
}

// buildFlappingHistoryEvent from a flapping history entry.
func (client *Client) buildFlappingHistoryEvent(ctx context.Context, h *v1history.FlappingHistory) (*fetchableEvent, error) {
	ev, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
	}

	if h.PercentStateChangeEnd.Valid {
		ev.Type = event.TypeFlappingEnd
		ev.Message = fmt.Sprintf(
			"Checkable stopped flapping (Current flapping value %.2f%% < low threshold %.2f%%)",
			h.PercentStateChangeEnd.Float64, h.FlappingThresholdLow)
		ev.Mute = types.MakeBool(false)
	} else if h.PercentStateChangeStart.Valid {
		ev.Type = event.TypeFlappingStart
		ev.Message = fmt.Sprintf(
			"Checkable started flapping (Current flapping value %.2f%% > high threshold %.2f%%)",
			h.PercentStateChangeStart.Float64, h.FlappingThresholdHigh)
		ev.Mute = types.MakeBool(true)
		ev.MuteReason = "Checkable is flapping"
	} else {
		return nil, errors.New("flapping history entry has neither percent_state_change_start nor percent_state_change_end")
	}

	return ev, nil
}

// buildAcknowledgementHistoryEvent from an acknowledgment history entry.
func (client *Client) buildAcknowledgementHistoryEvent(ctx context.Context, h *v1history.AcknowledgementHistory) (*fetchableEvent, error) {
	ev, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
	}

	if !h.ClearTime.Time().IsZero() {
		ev.Type = event.TypeAcknowledgementCleared
		ev.Message = "Acknowledgement was cleared"
		ev.Mute = types.MakeBool(false)

		if h.ClearedBy.Valid {
			ev.Username = h.ClearedBy.String
		}
	} else if !h.SetTime.Time().IsZero() {
		ev.Type = event.TypeAcknowledgementSet
		ev.Mute = types.MakeBool(true)
		ev.MuteReason = "Checkable was acknowledged"

		if h.Comment.Valid {
			ev.Message = h.Comment.String
		} else {
			ev.Message = "Checkable was acknowledged"
		}

		if h.Author.Valid {
			ev.Username = h.Author.String
		}
	} else {
		return nil, errors.New("acknowledgment history entry has neither a set_time nor a clear_time")
	}

	return ev, nil
}

// Submit this [database.Entity] to the Icinga Notifications API.
//
// Based on the entity's type, a different kind of event will be constructed. The event will be sent to the API in a
// blocking fashion.
//
// Returns true if this entity was processed or cannot be processed any further. Returns false if this entity should be
// retried later.
//
// This method uses the Client's logger.
func (client *Client) Submit(entity database.Entity) bool {
	if client.ctx.Err() != nil {
		client.logger.Errorw("Cannot process submitted entity as client context is done", zap.Error(client.ctx.Err()))
		return true
	}

	var (
		ev       *fetchableEvent
		eventErr error
	)

	// Keep the type switch in sync with the values of SyncKeyStructPtrs below.
	switch h := entity.(type) {
	case *v1history.AcknowledgementHistory:
		ev, eventErr = client.buildAcknowledgementHistoryEvent(client.ctx, h)

	case *v1history.DowntimeHistoryMeta:
		ev, eventErr = client.buildDowntimeHistoryMetaEvent(client.ctx, h)

	case *v1history.FlappingHistory:
		ev, eventErr = client.buildFlappingHistoryEvent(client.ctx, h)

	case *v1history.StateHistory:
		if h.StateType != common.HardState {
			return true
		}
		ev, eventErr = client.buildStateHistoryEvent(client.ctx, h)

	default:
		client.logger.Error("Cannot process unsupported type", zap.String("type", fmt.Sprintf("%T", h)))
		return true
	}

	if eventErr != nil {
		client.logger.Errorw("Cannot build event from history entry",
			zap.String("type", fmt.Sprintf("%T", entity)),
			zap.Error(eventErr))
		return true
	} else if ev == nil {
		// This really should not happen.
		client.logger.Errorw("No event was built, but no error was reported",
			zap.String("type", fmt.Sprintf("%T", entity)))
		return true
	}

	maxAttempts := 3
	for attempt := range maxAttempts {
		attributes, err := client.notificationsClient.ProcessEvent(client.ctx, ev.Event, true)
		if errors.Is(err, source.ErrAttrsNegotiation) {
			client.logger.Debugw("Icinga Notifications requested more attributes",
				zap.String("event", ev.Name),
				zap.Strings("attributes", attributes))

			if attempt == maxAttempts-1 {
				// Another completeAndUpdate call would be useless, as it won't be evaluated anymore.
				break
			}

			err := ev.completeAndUpdate(client.ctx, attributes)
			if err != nil {
				client.logger.Errorw("Cannot fetch required attribute for event",
					zap.String("event", ev.Name),
					zap.Strings("attributes", attributes),
					zap.Error(err))
				return false
			}
		} else if err != nil {
			client.logger.Errorw("Cannot submit event to Icinga Notifications",
				zap.String("event", ev.Name),
				zap.Error(err))
			return false
		} else {
			client.logger.Debugw("Successfully submitted event to Icinga Notifications", zap.String("event", ev.Name))
			return true
		}
	}

	client.logger.Warnw("Failed to submit event in three attempts", zap.String("event", ev.Name))
	return false
}

// SyncExtraStages returns a map of history sync keys to [history.StageFunc] to be used for [history.Sync].
//
// Passing the return value of this method as the extraStages parameter to [history.Sync] results in forwarding events
// from the Icinga DB history stream to Icinga Notifications after being resorted via the StreamSorter.
func (client *Client) SyncExtraStages() map[string]history.StageFunc {
	var syncKeyStructPtrs = map[string]any{
		history.SyncPipelineAcknowledgement: (*v1history.AcknowledgementHistory)(nil),
		history.SyncPipelineDowntime:        (*v1history.DowntimeHistoryMeta)(nil),
		history.SyncPipelineFlapping:        (*v1history.FlappingHistory)(nil),
		history.SyncPipelineState:           (*v1history.StateHistory)(nil),
	}

	sorterCallbackFn := func(msg redis.XMessage, key string) bool {
		makeEntity := func(key string, values map[string]any) (database.Entity, error) {
			structPtr, ok := syncKeyStructPtrs[key]
			if !ok {
				return nil, fmt.Errorf("key is not part of keyStructPtrs")
			}

			structifier := structify.MakeMapStructifier(
				reflect.TypeOf(structPtr).Elem(),
				"json",
				contracts.SafeInit)
			val, err := structifier(values)
			if err != nil {
				return nil, errors.Wrapf(err, "can't structify values %#v for %q", values, key)
			}

			entity, ok := val.(database.Entity)
			if !ok {
				return nil, fmt.Errorf("structifier returned %T which does not implement database.Entity", val)
			}

			return entity, nil
		}

		entity, err := makeEntity(key, msg.Values)
		if err != nil {
			client.logger.Errorw("Failed to create database.Entity out of Redis stream message",
				zap.Error(err),
				zap.String("key", key),
				zap.String("id", msg.ID))
			return false
		}

		success := client.Submit(entity)
		if success {
			telemetry.Stats.NotificationSync.Add(1)
		}
		return success
	}

	pipelineFn := NewStreamSorter(client.ctx, client.logger, sorterCallbackFn).PipelineFunc

	extraStages := make(map[string]history.StageFunc)
	for k := range syncKeyStructPtrs {
		extraStages[k] = pipelineFn
	}

	return extraStages
}
