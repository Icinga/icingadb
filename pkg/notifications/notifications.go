package notifications

import (
	"context"
	"crypto/sha1" // #nosec G505 -- Blocklisted import crypto/sha1
	"encoding/hex"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/objectpacker"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/structify"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	v1history "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

	// incidentsByObjId is a map of object IDs to incidents populated by the first call to ApplyDelta.
	incidentsByObjId map[string]source.Incident
	incidentsMu      sync.Mutex

	// heartbeatOutCh is a channel used to send heartbeat signals to the HA controller.
	heartbeatOutCh chan<- bool
}

// NewNotificationsClient creates a new Client connected to an existing database and logger.
func NewNotificationsClient(
	db *database.DB,
	rc *redis.Client,
	logger *logging.Logger,
	cfg source.Config,
	heartbeatOutCh chan<- bool,
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

		heartbeatOutCh: heartbeatOutCh,
	}, nil
}

// ClearIncidents clears the cached incidents previously populated by the [Client.ApplyDelta].
//
// This serves two purposes: it allows to free up memory when the incidents are no longer needed, and it allows
// to force a re-fetch of the incidents from the Icinga Notifications API on the next config dump, e.g. due to
// HA take over or the like.
//
// This function is not safe to call concurrently with [Client.ApplyDelta], but it is safe to call concurrently
// with itself. The [Client.ApplyDelta] function accesses the internal cache in a read-only manner without any
// synchronization once it has been populated, so you should ensure that all calls to [Client.ApplyDelta] have
// completed before calling this function.
func (client *Client) ClearIncidents() {
	client.incidentsMu.Lock()
	defer client.incidentsMu.Unlock()

	client.incidentsByObjId = nil
}

// ApplyDelta applies the given delta to the Icinga Notifications API.
//
// This function is called by the initial config dump after the delta has been calculated. It fetches the current
// incidents from the Icinga Notifications API and compares them with the delta. If there are any changes, it submits
// the new or updated events to the API and closes any incidents for deleted objects.
//
// Returns an error only when the provided context is canceled or Redis is unavailable. Otherwise, it will log any
// errors encountered during the submission of events to the Icinga Notifications API and continue processing the
// remaining events, but never returns an error for those.
func (client *Client) ApplyDelta(ctx context.Context, delta *icingadb.Delta) error {
	switch delta.Subject.Entity().(type) {
	case *v1.HostState, *v1.ServiceState:
	default:
		return nil
	}

	client.fetchIncidents(ctx)
	if len(client.incidentsByObjId) == 0 {
		return nil
	}

	client.logger.Infof("Fetching %d entities of type %s from Redis for submission to Icinga Notifications",
		len(delta.RedisSnapshot),
		delta.Subject.Name())

	g, ctx := errgroup.WithContext(ctx)
	pairs, errs := client.redisClient.HMYield(
		ctx,
		fmt.Sprintf("icinga:%s", strcase.Delimited(types.Name(delta.Subject.Entity()), ':')),
		slices.Collect(maps.Keys(delta.RedisSnapshot))...,
	)
	com.ErrgroupReceive(g, errs)

	entities, rErrs := icingaredis.CreateEntities(ctx, delta.Subject.Factory(), pairs, 1)
	com.ErrgroupReceive(g, rErrs)

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case entity, ok := <-entities:
				if !ok {
					return nil
				}

				if incident, exists := client.incidentsByObjId[entity.ID().String()]; exists {
					if same, err := HaveSameState(incident, entity); err != nil {
						return err
					} else if same {
						// Should we send a message update here too or just wait for the timer to expire
						// and sync the message accordingly? See https://github.com/Icinga/icingadb/issues/1140
						continue
					}
				}

				// If the entity is new or has a different state than the existing incident, submit it to Icinga
				// Notifications via the regular /process-event endpoint. Though, in case the API isn't healthy,
				// it will retry it endlessly, so set a timeout to avoid blocking the entire config dump.
				func() {
					ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
					defer cancel()
					_ = client.Submit(ctx, entity)
				}()
			}
		}
	})

	g.Go(func() error {
		var filter []any
		for id, incident := range client.incidentsByObjId {
			_, isServiceIncident := incident.ObjectTags["service"]
			_, isServiceState := delta.Subject.Entity().(*v1.ServiceState)

			_, deleted := delta.Delete[id]
			// We may have missed the initial config dump with the corresponding entity deletion, in which case we
			// can't rely on delta.Delete, delta.Create, or delta.Update to determine if the incident is obsolete.
			// Therefore, we also need to make sure that the corresponding object is still present in Redis.
			_, isInRedis := delta.RedisSnapshot[id]
			// Just because the object isn't in the Redis snapshot of this delta doesn't mean it's gone, but the
			// delta.RedisSnapshot simply reflects the state of Redis for a specific entity type, thus we can only
			// rely on it for the same entity type.
			if deleted || (!isInRedis && isServiceState == isServiceIncident) {
				filter = append(filter, incident.ObjectTags)
			}
		}
		if len(filter) == 0 {
			return nil
		}

		client.logger.Infof("Bulk closing %d obsolete incidents for deleted objects of type %s",
			len(filter),
			delta.Subject.Name())

		attrs := source.ModifiableIncidentAttrs{Close: types.MakeBool(true)}
		if err := client.notificationsClient.ModifyIncidents(ctx, attrs, filter); err != nil {
			client.logger.Errorw("Failed to bulk close obsolete incidents for deleted objects",
				zap.String("entity", delta.Subject.Name()),
				zap.Int("count", len(filter)),
				zap.String("error", err.Error()))
		}
		return nil
	})

	return g.Wait()
}

// fetchIncidents fetches all incidents from the Icinga Notifications API and stores them in the Client's incidentsByObjId map.
//
// If the incidents have already been fetched, this function does nothing. This function is safe to call concurrently.
func (client *Client) fetchIncidents(ctx context.Context) {
	client.incidentsMu.Lock()
	defer client.incidentsMu.Unlock()

	// ApplyDelta is called in parallel for each entity type, so we need to ensure
	// that we only fetch the incidents once per environment.
	if client.incidentsByObjId != nil {
		return
	}

	environment, ok := v1.EnvironmentFromContext(ctx)
	if !ok {
		panic("cannot get environment from context")
	}

	incidents, err := client.notificationsClient.GetIncidents(ctx, map[string]string{"environment": environment.ID().String()})
	if err != nil {
		client.sendHeartbeat(false)
		client.logger.Errorw("Failed to fetch incidents", zap.String("error", err.Error()))
		return
	}

	client.incidentsByObjId = make(map[string]source.Incident)
	hash := sha1.New() // #nosec G401 -- used as a non-cryptographic hash function to hash IDs
	for _, incident := range incidents {
		// This implementation mimics the Icinga 2 ID generation behavior[^1] used to generate all Icinga DB
		// related object IDs, so make sure to keep it in sync with the Icinga 2 implementation.
		//
		// [^1]: https://github.com/Icinga/icinga2/blob/v2.16.3/lib/icingadb/icingadb-utility.cpp#L81
		idTags := []string{environment.ID().String()}
		if service, ok := incident.ObjectTags["service"]; ok {
			idTags = append(idTags, incident.ObjectTags["host"]+"!"+service)
		} else {
			idTags = append(idTags, incident.ObjectTags["host"])
		}
		if err := objectpacker.PackAny(idTags, hash); err != nil {
			client.logger.Warnw("Cannot pack incident object ID for hashing, skipping incident",
				zap.Strings("id_tags", idTags),
				zap.Error(err))
			continue
		}
		client.incidentsByObjId[hex.EncodeToString(hash.Sum(nil))] = incident
		hash.Reset()
	}
}

// sendHeartbeat sends a heartbeat signal to the HA controller via the heartbeatOutCh channel.
func (client *Client) sendHeartbeat(alive bool) {
	select {
	case client.heartbeatOutCh <- alive:
	default:
		client.logger.Debugw("Heartbeat channel is full, dropping signal", zap.Bool("healthy", alive))
	}
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service IDs.
//
// This function is used by all event builders to create a common event structure that includes the host and service
// names, an Icinga DB Web reference, and the tags for the event.
// Any event type-specific information (like severity, message, etc.) is added by the specific event builders.
//
// The eventTime is used within Event.ID to distinguish two otherwise identical events.
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

	objectId := objectName + fmt.Sprintf("!%d", time.Now().UnixMilli())

	return &fetchableEvent{
		Event: &event.Event{
			ID:   objectId,
			Name: objectName,
			URL:  objectUrl,
			Tags: objectTags,
		},
		relations: rel,
	}, nil
}

// errNonVolatileNonHardState is returned when a non-hard state change is attempted to be submitted for a non-volatile checkable.
var errNonVolatileNonHardState = errors.New("non-hard state change for non-volatile checkable")

// buildStateEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (client *Client) buildStateEvent(ctx context.Context, s *v1.State, hostId, serviceId types.Binary) (*fetchableEvent, error) {
	ev, err := client.buildCommonEvent(ctx, hostId, serviceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostId, serviceId)
	}

	ev.Tags["environment"] = s.EnvironmentId.String()
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

	// If the checkable is volatile, it's always treated as a hard state change, but `StateType` is still set
	// to `SOFT` due to an Icinga 2 bug (see https://github.com/Icinga/icinga2/issues/10879).
	if s.StateType != common.HardState && !isVolatile {
		return nil, errNonVolatileNonHardState
	}

	if sev, err := StateToSeverity(s, serviceId != nil); err != nil {
		return nil, err
	} else {
		ev.Severity = sev
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
			ev.MutedReason += ", and an acknowledgement"
		} else if isAcked {
			ev.MutedReason += " an acknowledgement"
		}
		if isFlapping && (inDowntime || isAcked) {
			ev.MutedReason += ", and flapping as well"
		} else if isFlapping {
			ev.MutedReason += " flapping state"
		}
		ev.MutedReason += "."
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
	defer func() { panic("downtime history event generation is incomplete and not yet implemented") }()

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
	defer func() { panic("flapping history event generation is incomplete and not yet implemented") }()

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
	defer func() { panic("acknowledgement history event generation is incomplete and not yet implemented") }()

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
// error occurs (like ctx cancellation) or the deadline is exceeded. In other words, when this method returns an error,
// then it usually means that there's nothing it can do anymore to successfully submit the event, thus it should be
// treated as a fatal error.
//
// Note that this function is used as [icingadb.RUUpsertFunc] for the runtime updates pipeline, so its signature must
// match the [icingadb.RUUpsertFunc] type.
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
			zap.String("error", err.Error()))
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
					return err
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
		func(err error) bool { return true }, // Retry all errors.
		backoff.DefaultBackoff,
		retry.Settings{
			OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
				client.sendHeartbeat(true)
				telemetry.Stats.NotificationSync.Add(1)

				client.logger.Debugw("Successfully submitted event to Icinga Notifications",
					zap.String("event", ev.Name),
					zap.Uint64("attempt", attempt),
					zap.Duration("elapsed", elapsed),
					zap.Error(lastErr))
			},
			OnRetryableError: func(elapsed time.Duration, attempt uint64, err, lastErr error) {
				client.sendHeartbeat(false)

				if lastErr == nil || err.Error() != lastErr.Error() {
					client.logger.Errorw("Cannot submit event to Icinga Notifications",
						zap.String("event", ev.Name),
						zap.Uint64("attempt", attempt),
						zap.Duration("elapsed", elapsed),
						zap.Error(err))
				}
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

// StateToSeverity converts a state integer to an event.Severity value.
func StateToSeverity(s *v1.State, isService bool) (event.Severity, error) {
	if isService {
		switch s.HardState {
		case 0:
			return event.SeverityOK, nil
		case 1:
			return event.SeverityWarning, nil
		case 2:
			return event.SeverityCrit, nil
		case 3:
			return event.SeverityErr, nil
		default:
			return event.SeverityNone, fmt.Errorf("unexpected service state %d", s.HardState)
		}
	} else {
		switch s.HardState {
		case 0:
			return event.SeverityOK, nil
		case 1:
			return event.SeverityCrit, nil
		default:
			return event.SeverityNone, fmt.Errorf("unexpected host state %d", s.HardState)
		}
	}
}

// HaveSameState checks if the given incident and the corresponding [database.Entity] have the same state.
//
// This function is used to determine if an incident in Icinga Notifications corresponds to the current state
// of a checkable in Icinga DB. It compares the severity and muted status of the incident with the state of the
// entity and returns true if they match, false otherwise. If the entity type is unsupported, an error is returned.
func HaveSameState(incident source.Incident, entity database.Entity) (bool, error) {
	var s *v1.State
	var isService bool
	switch e := entity.(type) {
	case *v1.HostState:
		s = &e.State
	case *v1.ServiceState:
		s = &e.State
		isService = true
	default:
		return false, fmt.Errorf("unsupported entity type %T", entity)
	}

	severity, err := StateToSeverity(s, isService)
	if err != nil {
		return false, err
	}

	if incident.Severity != severity {
		return false, nil
	}

	inDowntime := s.InDowntime.Valid && s.InDowntime.Bool
	isAcked := s.IsAcknowledged.Valid && s.IsAcknowledged.Bool
	isFlapping := s.IsFlapping.Valid && s.IsFlapping.Bool
	if incident.IsMuted != (inDowntime || isAcked || isFlapping) {
		return false, nil
	}
	return true, nil
}
