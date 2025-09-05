package notifications

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1history "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"slices"
	"strings"
)

// submission of a [database.Entity] to the Client.
type submission struct {
	entity database.Entity
	traces map[string]time.Time
}

// MarshalLogObject implements [zapcore.ObjectMarshaler] to print a debug trace.
func (sub submission) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("type", fmt.Sprintf("%T", sub.entity))

	if len(sub.traces) < 1 {
		return nil
	}

	tracesKeys := slices.SortedFunc(func(yield func(string) bool) {
		for key := range sub.traces {
			if !yield(key) {
				return
			}
		}
	}, func(a string, b string) int {
		return sub.traces[a].Compare(sub.traces[b])
	})

	relTraces := make([]string, 0, len(tracesKeys)-1)
	for i := 1; i < len(tracesKeys); i++ {
		relTraces = append(relTraces, fmt.Sprintf("%s: %v",
			tracesKeys[i],
			sub.traces[tracesKeys[i]].Sub(sub.traces[tracesKeys[i-1]])))
	}

	encoder.AddDuration("processing_time", sub.traces[tracesKeys[len(tracesKeys)-1]].Sub(sub.traces[tracesKeys[0]]))
	encoder.AddString("trace", strings.Join(relTraces, ", "))

	return nil
}

// Client is an Icinga Notifications compatible client implementation to push events to Icinga Notifications.
//
// A new Client should be created by the NewNotificationsClient function. New history entries can be submitted by
// calling the Source.Submit method. The Client will then process the history entries in a background worker goroutine.
type Client struct {
	source.Config

	inputCh chan submission // inputCh is a buffered channel used to submit history entries to the worker.
	db      *database.DB
	logger  *logging.Logger

	rules *source.RulesInfo // rules holds the latest rules fetched from Icinga Notifications.

	ctx       context.Context
	ctxCancel context.CancelFunc

	notificationsClient *source.Client // The Icinga Notifications client used to interact with the API.
	redisClient         *redis.Client  // redisClient is the Redis client used to fetch host and service names for events.
}

// NewNotificationsClient creates a new Client connected to an existing database and logger.
//
// This function starts a worker goroutine in the background which can be stopped by ending the provided context.
func NewNotificationsClient(
	ctx context.Context,
	db *database.DB,
	rc *redis.Client,
	logger *logging.Logger,
	cfg source.Config,
) *Client {
	ctx, ctxCancel := context.WithCancel(ctx)

	client := &Client{
		Config: cfg,

		inputCh: make(chan submission, 1<<10), // chosen by fair dice roll
		db:      db,
		logger:  logger,

		rules:       &source.RulesInfo{Version: source.EmptyRulesVersion},
		redisClient: rc,

		ctx:       ctx,
		ctxCancel: ctxCancel,
	}

	notificationsClient, err := source.NewClient(client.Config, "Icinga DB")
	if err != nil {
		logger.Fatalw("Cannot create Icinga Notifications client", zap.Error(err))
	}
	client.notificationsClient = notificationsClient

	go client.worker()

	return client
}

// evaluateRulesForObject returns the rule IDs for each matching query.
//
// At the moment, each RuleResp.ObjectFilterExpr is executed as a SQL query after the parameters are being bound. If the
// query returns at least one line, the rule will match. Rules with an empty ObjectFilterExpr are a special case and
// will always match.
//
// The provided entity is passed as param to the queries, thus they are allowed to use all fields of that specific
// entity. Cross-table column references are not supported unless the provided entity provides the fields in one way
// or another.
//
// This allows a query like the following:
//
//	> select * from host where id = :host_id and environment_id = :environment_id and name like 'prefix_%'
//
// The :host_id and :environment_id parameters will be bound to the entity's ID and EnvironmentId fields, respectively.
func (s *Client) evaluateRulesForObject(ctx context.Context, entity database.Entity) ([]int64, error) {
	outRuleIds := make([]int64, 0, len(s.rules.Rules))

	for rule := range s.rules.Iter() {
		if rule.ObjectFilterExpr == "" {
			outRuleIds = append(outRuleIds, rule.Id)
			continue
		}

		evaluates, err := func() (bool, error) {
			// The raw SQL query in the database is URL-encoded (mostly the space character is replaced by %20).
			// So, we need to unescape it before passing it to the database.
			query, err := url.QueryUnescape(rule.ObjectFilterExpr)
			if err != nil {
				return false, errors.Wrapf(err, "cannot unescape rule %d object filter expression %q", rule.Id, rule.ObjectFilterExpr)
			}
			rows, err := s.db.NamedQueryContext(ctx, s.db.Rebind(query), entity)
			if err != nil {
				return false, err
			}
			defer func() { _ = rows.Close() }()

			if !rows.Next() {
				return false, nil
			}
			return true, nil
		}()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot fetch rule %d from %q", rule.Id, rule.ObjectFilterExpr)
		} else if !evaluates {
			continue
		}
		outRuleIds = append(outRuleIds, rule.Id)
	}

	return outRuleIds, nil
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service names.
//
// This function is used by all event builders to create a common event structure that includes the host and service
// names, the absolute URL to the Icinga Web 2 Icinga DB page for the host or service, and the tags for the event.
// Any event type-specific information (like severity, message, etc.) is added by the specific event builders.
func (s *Client) buildCommonEvent(rlr *redisLookupResult) (*event.Event, error) {
	var (
		objectName string
		objectUrl  *url.URL
		objectTags map[string]string
	)

	if rlr.ServiceName != "" {
		objectName = rlr.HostName + "!" + rlr.ServiceName

		objectUrl = s.notificationsClient.JoinIcingaWeb2Path("/icingadb/service")
		objectUrl.RawQuery = "name=" + utils.RawUrlEncode(rlr.ServiceName) + "&host.name=" + utils.RawUrlEncode(rlr.HostName)

		objectTags = map[string]string{
			"host":    rlr.HostName,
			"service": rlr.ServiceName,
		}
	} else {
		objectName = rlr.HostName

		objectUrl = s.notificationsClient.JoinIcingaWeb2Path("/icingadb/host")
		objectUrl.RawQuery = "name=" + utils.RawUrlEncode(rlr.HostName)

		objectTags = map[string]string{
			"host": rlr.HostName,
		}
	}

	return &event.Event{
		Name: objectName,
		URL:  objectUrl.String(),
		Tags: objectTags,
	}, nil
}

// buildStateHistoryEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (s *Client) buildStateHistoryEvent(ctx context.Context, h *v1history.StateHistory) (*event.Event, error) {
	res, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, err
	}

	ev, err := s.buildCommonEvent(res)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", res.HostName, res.ServiceName)
	}

	ev.Type = event.TypeState

	if res.ServiceName != "" {
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
func (s *Client) buildDowntimeHistoryMetaEvent(ctx context.Context, h *v1history.DowntimeHistoryMeta) (*event.Event, error) {
	res, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, err
	}

	ev, err := s.buildCommonEvent(res)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", res.HostName, res.ServiceName)
	}

	switch h.EventType {
	case "downtime_start":
		ev.Type = event.TypeDowntimeStart
		ev.Username = h.Author
		ev.Message = h.Comment
		ev.Mute = types.MakeBool(true)
		ev.MuteReason = "Checkable is in downtime"

	case "downtime_end":
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
func (s *Client) buildFlappingHistoryEvent(ctx context.Context, h *v1history.FlappingHistory) (*event.Event, error) {
	res, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, err
	}

	ev, err := s.buildCommonEvent(res)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", res.HostName, res.ServiceName)
	}

	if h.PercentStateChangeEnd.Valid {
		ev.Type = event.TypeFlappingEnd
		ev.Message = fmt.Sprintf(
			"Checkable stopped flapping (Current flapping value %.2f%% < low threshold %.2f%%)",
			h.PercentStateChangeEnd.Float64, h.FlappingThresholdLow)
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
func (s *Client) buildAcknowledgementHistoryEvent(ctx context.Context, h *v1history.AcknowledgementHistory) (*event.Event, error) {
	res, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, err
	}

	ev, err := s.buildCommonEvent(res)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", res.HostName, res.ServiceName)
	}

	if !h.ClearTime.Time().IsZero() {
		ev.Type = event.TypeAcknowledgementCleared
		ev.Message = "Checkable was cleared"

		if h.ClearedBy.Valid {
			ev.Username = h.ClearedBy.String
		}
	} else if !h.SetTime.Time().IsZero() {
		ev.Type = event.TypeAcknowledgementSet

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

// worker is the background worker launched by NewNotificationsClient.
func (s *Client) worker() {
	defer s.ctxCancel()

	for {
		select {
		case <-s.ctx.Done():
			return

		case sub, more := <-s.inputCh:
			if !more { // Should never happen, but just in case.
				s.logger.Debug("Input channel closed, stopping worker")
				return
			}

			sub.traces["worker_start"] = time.Now()

			var ev *event.Event
			var eventErr error

			// Keep the type switch in sync with syncPipelines from pkg/icingadb/history/sync.go
			switch h := sub.entity.(type) {
			case *v1history.AcknowledgementHistory:
				ev, eventErr = s.buildAcknowledgementHistoryEvent(s.ctx, h)

			case *v1history.DowntimeHistoryMeta:
				ev, eventErr = s.buildDowntimeHistoryMetaEvent(s.ctx, h)

			case *v1history.FlappingHistory:
				ev, eventErr = s.buildFlappingHistoryEvent(s.ctx, h)

			case *v1history.StateHistory:
				if h.StateType != common.HardState {
					continue
				}
				ev, eventErr = s.buildStateHistoryEvent(s.ctx, h)

			default:
				s.logger.Error("Cannot process unsupported type",
					zap.Object("submission", sub),
					zap.String("type", fmt.Sprintf("%T", h)))
				continue
			}

			if eventErr != nil {
				s.logger.Errorw("Cannot build event from history entry",
					zap.Object("submission", sub),
					zap.String("type", fmt.Sprintf("%T", sub.entity)),
					zap.Error(eventErr))
				continue
			} else if ev == nil {
				// This really should not happen.
				s.logger.Errorw("No event was fetched, but no error was reported.", zap.Object("submission", sub))
				continue
			}

			eventLogger := s.logger.With(zap.Object(
				"event",
				zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
					encoder.AddString("name", ev.Name)
					encoder.AddString("type", ev.Type.String())
					return nil
				}),
			))

			sub.traces["evaluate_jump_pre"] = time.Now()
		reevaluateRules:
			sub.traces["evaluate_jump_last"] = time.Now()
			eventRuleIds, err := s.evaluateRulesForObject(s.ctx, sub.entity)
			if err != nil {
				eventLogger.Errorw("Cannot evaluate rules for event",
					zap.Object("submission", sub),
					zap.Error(err))
				continue
			}

			sub.traces["process_last"] = time.Now()
			newEventRules, err := s.notificationsClient.ProcessEvent(s.ctx, ev, s.rules.Version, eventRuleIds...)
			if errors.Is(err, source.ErrRulesOutdated) {
				s.rules = newEventRules

				eventLogger.Infow("Re-evaluating rules for event after fetching new rules",
					zap.Object("submission", sub),
					zap.String("rules_version", s.rules.Version))

				// Re-evaluate the just fetched rules for the current event.
				goto reevaluateRules
			} else if err != nil {
				eventLogger.Errorw("Cannot submit event to Icinga Notifications",
					zap.Object("submission", sub),
					zap.String("rules_version", s.rules.Version),
					zap.Any("rules", eventRuleIds),
					zap.Error(err))
				continue
			}

			sub.traces["worker_fin"] = time.Now()
			eventLogger.Debugw("Successfully submitted event to Icinga Notifications",
				zap.Object("submission", sub),
				zap.Any("rules", eventRuleIds))
		}
	}
}

// Submit a history entry to be processed by the Client's internal worker loop.
//
// Internally, a buffered channel is used for delivery. So this function should not block. Otherwise, it will abort
// after a second and an error is logged.
func (s *Client) Submit(entity database.Entity) {
	sub := submission{
		entity: entity,
		traces: map[string]time.Time{
			"submit": time.Now(),
		},
	}

	select {
	case <-s.ctx.Done():
		s.logger.Errorw("Client context is done, rejecting submission",
			zap.Object("submission", sub),
			zap.Error(s.ctx.Err()))
		return

	case s.inputCh <- sub:
		return

	case <-time.After(time.Second):
		s.logger.Error("Client submission channel is blocking, rejecting submission",
			zap.Object("submission", sub))
		return
	}
}

var SyncKeyStructPtrs = map[string]any{
	history.SyncPipelineAcknowledgement: (*v1history.AcknowledgementHistory)(nil),
	history.SyncPipelineDowntime:        (*v1history.DowntimeHistoryMeta)(nil),
	history.SyncPipelineFlapping:        (*v1history.FlappingHistory)(nil),
	history.SyncPipelineState:           (*v1history.StateHistory)(nil),
}
