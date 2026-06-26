package notifications

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/structify"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
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

	notificationsClient *source.Client // The Icinga Notifications client used to interact with the API.
	redisClient         *redis.Client  // redisClient is the Redis client used to fetch host and service names for events.
}

// NewNotificationsClient creates a new Client connected to an existing database and logger.
func NewNotificationsClient(
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

	return &fetchableEvent{
		Event: &event.Event{
			Name: objectName,
			URL:  objectUrl,
			Tags: objectTags,
		},
		relations: rel,
	}, nil
}

// errNonVolatileNonHardState is returned when a non-hard state change is attempted to be submitted for a non-volatile checkable.
var errNonVolatileNonHardState = errors.New("non-hard state change for non-volatile checkable")

// buildStateHistoryEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (client *Client) buildStateEvent(ctx context.Context, s *v1.State, hostId, serviceId types.Binary) (*fetchableEvent, error) {
	ev, err := client.buildCommonEvent(ctx, hostId, serviceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostId, serviceId)
	}

	if s.Output.Valid {
		ev.Message = s.Output.String
	}
	if s.LongOutput.Valid {
		ev.Message += "\n" + s.LongOutput.String
	}

	var isVolatile bool
	if serviceId != nil {
		isVolatile = ev.Services[0].isVolatile
	} else {
		isVolatile = ev.Host.isVolatile
	}

	// For non-hard state changes, we want to just update the event message without affecting any other incident state.
	// However, if the checkable is volatile, it's always treated as a hard state change, it's still set to `SOFT` due
	// to an Icinga 2 bug (see https://github.com/Icinga/icinga2/issues/10879).
	if s.StateType != common.HardState && !isVolatile {
		return nil, errNonVolatileNonHardState
	}

	if serviceId != nil {
		switch s.HardState {
		case 0:
			ev.Severity = event.SeverityOK
		case 1:
			ev.Severity = event.SeverityWarning
		case 2:
			ev.Severity = event.SeverityCrit
		case 3:
			ev.Severity = event.SeverityErr
		default:
			return nil, fmt.Errorf("unexpected service state %d", s.HardState)
		}
	} else {
		switch s.HardState {
		case 0:
			ev.Severity = event.SeverityOK
		case 1:
			ev.Severity = event.SeverityCrit
		default:
			return nil, fmt.Errorf("unexpected host state %d", s.HardState)
		}
	}

	inDowntime := s.InDowntime.Valid && s.InDowntime.Bool
	isAcked := s.IsAcknowledged.Valid && s.IsAcknowledged.Bool
	isFlapping := s.IsFlapping.Valid && s.IsFlapping.Bool
	ev.Muted = types.MakeBool(inDowntime || isAcked || isFlapping)
	if ev.IsMuted() {
		ev.MutedReason = "Checkable is muted due to"
		if inDowntime {
			ev.MutedReason += " currently active downtime"
		}
		if isAcked && inDowntime {
			ev.MutedReason += ", and acknowledgement"
		} else if isAcked {
			ev.MutedReason += " an acknowledgement"
		}
		if isFlapping && (inDowntime || isAcked) {
			ev.MutedReason += ", and flapping as well."
		} else if isFlapping {
			ev.MutedReason += " flapping state"
		}
	} else {
		ev.MutedReason = "Checkable is not muted (no active downtime, no acknowledgement, and not flapping)"
	}

	ev.Incident = types.MakeBool(true)
	if ev.Severity == event.SeverityOK && !ev.IsMuted() {
		// If the object is still muted, we don't close incidents even with OK state changes.
		// See https://github.com/Icinga/icingadb/issues/1127#issuecomment-4691435590 for details.
		ev.Close = types.MakeBool(true)
	} else if s.PreviousHardState == s.HardState {
		// NON-OK hard state changes that do not change the state are volatile ones, so set the notify flag.
		ev.Notify = types.MakeBool(true)
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
		ev.Message = h.Comment

	case "downtime_end":
		if h.HasBeenCancelled.Valid && h.HasBeenCancelled.Bool {
			ev.Message = "Downtime was cancelled"
			if h.CancelledBy.Valid {
				ev.Message += " (cancelled by " + h.CancelledBy.String + ")"
			}
		} else {
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
		ev.Message = fmt.Sprintf(
			"Checkable stopped flapping (Current flapping value %.2f%% < low threshold %.2f%%)",
			h.PercentStateChangeEnd.Float64, h.FlappingThresholdLow)
	} else if h.PercentStateChangeStart.Valid {
		ev.Message = fmt.Sprintf(
			"Checkable started flapping (Current flapping value %.2f%% > high threshold %.2f%%)",
			h.PercentStateChangeStart.Float64, h.FlappingThresholdHigh)
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
		ev.Message = "Acknowledgement was cleared"
		if h.ClearedBy.Valid {
			ev.Message += " (cleared by " + h.ClearedBy.String + ")"
		}
	} else if !h.SetTime.Time().IsZero() {
		if h.Comment.Valid {
			ev.Message = h.Comment.String
		} else {
			ev.Message = "Checkable was acknowledged"
		}
	} else {
		return nil, errors.New("acknowledgment history entry has neither a set_time nor a clear_time")
	}

	return ev, nil
}

// Submit this [database.Entity] to the Icinga Notifications API.
//
// Based on the entity's type, a different kind of event will be constructed. The event will be sent to the API in a
// blocking fashion and will be retried with an exponential backoff in case of retryable errors until a non-retryable
// error occurs (like ctx cancellation) or the deadline is exceeded. In other words, when this method only returns
// an error, then it usually means that there's nothing it can do anymore to successfully submit the event, thus it
// should be treated as a fatal error.
func (client *Client) Submit(ctx context.Context, entity database.Entity) error {
	var (
		ev       *fetchableEvent
		eventErr error
	)

	canIgnoreStateUpdate := func(s *v1.State) bool {
		// Ignore PENDING -> OK, otherwise we'll have a bunch of incidents that are be closed immediately.
		// Also ignore any Pending states (99), as these are not relevant for notifications.
		return s.HardState == 99 || (s.HardState == 0 && s.PreviousHardState == 99)
	}

	// Keep the type switch in sync with the values of SyncKeyStructPtrs below.
	switch h := entity.(type) {
	case *v1history.AcknowledgementHistory:
		ev, eventErr = client.buildAcknowledgementHistoryEvent(ctx, h)

	case *v1history.DowntimeHistoryMeta:
		ev, eventErr = client.buildDowntimeHistoryMetaEvent(ctx, h)

	case *v1history.FlappingHistory:
		ev, eventErr = client.buildFlappingHistoryEvent(ctx, h)

	case *v1.HostState:
		if canIgnoreStateUpdate(&h.State) {
			return nil
		}
		ev, eventErr = client.buildStateEvent(ctx, &h.State, h.Id, nil)

	case *v1.ServiceState:
		if canIgnoreStateUpdate(&h.State) {
			return nil
		}
		ev, eventErr = client.buildStateEvent(ctx, &h.State, h.HostId, h.ServiceId)

	case *v1.DependencyEdgeState, *v1.RedundancygroupState:
		// Nothing to do here, we only received these because they're part of the runtime state update pipeline.
		return nil

	default:
		client.logger.Errorw("Cannot process unsupported type", zap.String("type", fmt.Sprintf("%T", h)))
		return nil
	}

	if eventErr != nil {
		if !errors.Is(eventErr, errNonVolatileNonHardState) {
			client.logger.Errorw("Cannot build event for entity, skipping submission",
				zap.String("type", fmt.Sprintf("%T", entity)),
				zap.Error(eventErr))
		}
		return nil
	} else if ev == nil {
		// This really should not happen.
		client.logger.Errorw("No event was built, but no error was reported",
			zap.String("type", fmt.Sprintf("%T", entity)))
		return nil
	}

	if err := ev.Validate(); err != nil {
		client.logger.Errorw("BUG: generated event is invalid, skipping submission",
			zap.Any("event", ev.Event),
			zap.Any("entity", entity),
			zap.Error(err))
		return nil
	}

	attributes := client.DefaultRelations
	return retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			for {
				if err := ev.completeAndUpdate(ctx, attributes); err != nil {
					client.logger.Errorw("Cannot fetch required attribute for event",
						zap.String("event", ev.Name),
						zap.Strings("attributes", attributes),
						zap.Error(err))

					// ev.completeAndUpdate retries any retryable errors internally, so if we get an error here,
					// it means that we cannot complete the event with the given attributes, so we should not retry
					// anymore. Therefore, we return a non-retryable error here instead of propagating the original one.
					return errors.New("cannot complete event relations")
				}

				attributes, err = client.notificationsClient.ProcessEvent(ctx, ev.Event, true)
				if errors.Is(err, source.ErrAttrsNegotiation) {
					client.logger.Debugw("Icinga Notifications requested more attributes",
						zap.String("event", ev.Name),
						zap.Strings("attributes", attributes))
					continue
				}
				return err
			}
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		retry.Settings{
			Timeout: retry.DefaultTimeout,
			OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
				telemetry.Stats.NotificationSync.Add(1)

				client.logger.Debugw("Successfully submitted event to Icinga Notifications",
					zap.String("event", ev.Name),
					zap.Uint64("attempt", attempt),
					zap.Duration("elapsed", elapsed),
					zap.Error(lastErr))
			},
			OnRetryableError: func(elapsed time.Duration, attempt uint64, err, lastErr error) {
				client.logger.Warnw("Cannot submit event to Icinga Notifications",
					zap.String("event", ev.Name),
					zap.Uint64("attempt", attempt),
					zap.Duration("elapsed", elapsed),
					zap.Error(err))
			},
		},
	)
}

// SyncExtraStages returns a map of history sync keys to [history.StageFunc] to be used for [history.Sync].
//
// Passing the return value of this method as the extraStages parameter to [history.Sync] results in forwarding events
// from the Icinga DB history stream to Icinga Notifications after being resorted via the StreamSorter.
func (client *Client) SyncExtraStages(ctx context.Context) map[string]history.StageFunc {
	var syncKeyStructPtrs = map[string]any{
		history.SyncPipelineAcknowledgement: (*v1history.AcknowledgementHistory)(nil),
		history.SyncPipelineDowntime:        (*v1history.DowntimeHistoryMeta)(nil),
		history.SyncPipelineFlapping:        (*v1history.FlappingHistory)(nil),
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

		if err := client.Submit(ctx, entity); err == nil {
			return true
		}
		return false
	}

	pipelineFn := NewStreamSorter(ctx, client.logger, sorterCallbackFn).PipelineFunc

	extraStages := make(map[string]history.StageFunc)
	for k := range syncKeyStructPtrs {
		extraStages[k] = pipelineFn
	}

	return extraStages
}
