package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/pkg/common"
	v1history "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Source is an Icinga Notifications compatible source implementation to push events to Icinga Notifications.
//
// A new Source should be created by the NewNotificationsSource function. New history entries can be submitted by
// calling the Source.Submit method. The Source will then process the history entries in a background worker goroutine.
type Source struct {
	notifications.Config

	inputCh chan database.Entity // inputCh is a buffered channel used to submit history entries to the worker.
	db      *database.DB
	logger  *logging.Logger

	rules      *notifications.SourceRulesInfo // rules holds the latest rules fetched from Icinga Notifications.
	rulesMutex sync.RWMutex                   // rulesMutex protects access to the rules field.

	ctx       context.Context
	ctxCancel context.CancelFunc

	notificationsClient *notifications.Client // The Icinga Notifications client used to interact with the API.
	redisClient         *redis.Client         // redisClient is the Redis client used to fetch host and service names for events.
}

// NewNotificationsSource creates a new Source connected to an existing database and logger.
//
// This function starts a worker goroutine in the background which can be stopped by ending the provided context.
func NewNotificationsSource(
	ctx context.Context,
	db *database.DB,
	rc *redis.Client,
	logger *logging.Logger,
	cfg notifications.Config,
) *Source {
	ctx, ctxCancel := context.WithCancel(ctx)

	source := &Source{
		Config: cfg,

		inputCh: make(chan database.Entity, 1<<10), // chosen by fair dice roll
		db:      db,
		logger:  logger,

		rules:       &notifications.SourceRulesInfo{Version: notifications.EmptyRulesVersion},
		redisClient: rc,

		ctx:       ctx,
		ctxCancel: ctxCancel,
	}

	client, err := notifications.NewClient(source.Config, "Icinga DB")
	if err != nil {
		logger.Fatalw("Cannot create Icinga Notifications client", zap.Error(err))
	}
	source.notificationsClient = client

	go source.worker()

	return source
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
func (s *Source) evaluateRulesForObject(ctx context.Context, entity database.Entity) ([]int64, error) {
	s.rulesMutex.RLock()
	defer s.rulesMutex.RUnlock()

	outRuleIds := make([]int64, 0, len(s.rules.Rules))

	for rule := range s.rules.Iter() {
		if rule.ObjectFilterExpr == "" {
			outRuleIds = append(outRuleIds, rule.Id)
			continue
		}

		run := func() error {
			// The raw SQL query in the database is URL-encoded (mostly the space character is replaced by %20).
			// So, we need to unescape it before passing it to the database.
			query, err := url.QueryUnescape(rule.ObjectFilterExpr)
			if err != nil {
				return errors.Wrapf(err, "cannot unescape rule %d object filter expression %q", rule.Id, rule.ObjectFilterExpr)
			}
			rows, err := s.db.NamedQueryContext(ctx, s.db.Rebind(query), entity)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()

			if !rows.Next() {
				return sql.ErrNoRows
			}
			return nil
		}

		if err := run(); err == nil {
			outRuleIds = append(outRuleIds, rule.Id)
		} else if errors.Is(err, sql.ErrNoRows) {
			continue
		} else {
			return nil, errors.Wrapf(err, "cannot fetch rule %d from %q", rule.Id, rule.ObjectFilterExpr)
		}
	}

	return outRuleIds[:len(outRuleIds):len(outRuleIds)], nil
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service names.
//
// This function is used by all event builders to create a common event structure that includes the host and service
// names, the absolute URL to the Icinga Web 2 Icinga DB page for the host or service, and the tags for the event.
// Any event type-specific information (like severity, message, etc.) is added by the specific event builders.
func (s *Source) buildCommonEvent(rlr *redisLookupResult) (*event.Event, error) {
	var (
		eventName string
		eventUrl  *url.URL
		eventTags map[string]string
	)

	if rlr.ServiceName != "" {
		eventName = rlr.HostName + "!" + rlr.ServiceName

		eventUrl = s.notificationsClient.JoinIcingaWeb2Path("/icingadb/service")
		eventUrl.RawQuery = "name=" + utils.RawUrlEncode(rlr.ServiceName) + "&host.name=" + utils.RawUrlEncode(rlr.HostName)

		eventTags = map[string]string{
			"host":    rlr.HostName,
			"service": rlr.ServiceName,
		}
	} else {
		eventName = rlr.HostName

		eventUrl = s.notificationsClient.JoinIcingaWeb2Path("/icingadb/host")
		eventUrl.RawQuery = "name=" + utils.RawUrlEncode(rlr.HostName)

		eventTags = map[string]string{
			"host": rlr.HostName,
		}
	}

	return &event.Event{
		Name: eventName,
		URL:  eventUrl.String(),
		Tags: eventTags,
	}, nil
}

// buildStateHistoryEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (s *Source) buildStateHistoryEvent(ctx context.Context, h *v1history.StateHistory) (*event.Event, error) {
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

// buildDowntimeHistoryEvent from a downtime history entry.
func (s *Source) buildDowntimeHistoryEvent(ctx context.Context, h *v1history.DowntimeHistory) (*event.Event, error) {
	res, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, err
	}

	ev, err := s.buildCommonEvent(res)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", res.HostName, res.ServiceName)
	}

	if h.HasBeenCancelled.Valid && h.HasBeenCancelled.Bool {
		ev.Type = event.TypeDowntimeRemoved
		ev.Message = "Downtime was cancelled"

		if h.CancelledBy.Valid {
			ev.Username = h.CancelledBy.String
		}
	} else if h.EndTime.Time().Compare(time.Now()) <= 0 {
		ev.Type = event.TypeDowntimeEnd
		ev.Message = "Downtime expired"
	} else {
		ev.Type = event.TypeDowntimeStart
		ev.Username = h.Author
		ev.Message = h.Comment
		ev.Mute = types.MakeBool(true)
		ev.MuteReason = "Checkable is in downtime"
	}

	return ev, nil
}

// buildFlappingHistoryEvent from a flapping history entry.
func (s *Source) buildFlappingHistoryEvent(ctx context.Context, h *v1history.FlappingHistory) (*event.Event, error) {
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
func (s *Source) buildAcknowledgementHistoryEvent(ctx context.Context, h *v1history.AcknowledgementHistory) (*event.Event, error) {
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

// worker is the background worker launched by NewNotificationsSource.
func (s *Source) worker() {
	defer s.ctxCancel()

	for {
		select {
		case <-s.ctx.Done():
			return

		case entity, more := <-s.inputCh:
			if !more { // Should never happen, but just in case.
				s.logger.Debug("Input channel closed, stopping worker")
				return
			}

			var ev *event.Event
			var eventErr error

			// Keep the type switch in sync with syncPipelines from pkg/icingadb/history/sync.go
			switch h := entity.(type) {
			case *v1history.NotificationHistory:
				// Ignore for the moment.
				continue

			case *v1history.StateHistory:
				if h.StateType != common.HardState {
					continue
				}

				ev, eventErr = s.buildStateHistoryEvent(s.ctx, h)

			case *v1history.DowntimeHistory:
				ev, eventErr = s.buildDowntimeHistoryEvent(s.ctx, h)

			case *v1history.CommentHistory:
				// Ignore for the moment.
				continue

			case *v1history.FlappingHistory:
				ev, eventErr = s.buildFlappingHistoryEvent(s.ctx, h)

			case *v1history.AcknowledgementHistory:
				ev, eventErr = s.buildAcknowledgementHistoryEvent(s.ctx, h)

			default:
				s.logger.Error("Cannot process unsupported type",
					zap.String("type", fmt.Sprintf("%T", h)))
				continue
			}

			if eventErr != nil {
				s.logger.Errorw("Cannot build event from history entry",
					zap.String("type", fmt.Sprintf("%T", entity)),
					zap.Error(eventErr))
				continue
			}
			if ev == nil {
				s.logger.Error("No event was fetched, but no error was reported. This REALLY SHOULD NOT happen.")
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

		reevaluateRules:
			eventRuleIds, err := s.evaluateRulesForObject(s.ctx, entity)
			if err != nil {
				eventLogger.Errorw("Cannot evaluate rules for event", zap.Error(err))
				continue
			}

			s.rulesMutex.RLock()
			ruleVersion := s.rules.Version
			s.rulesMutex.RUnlock()

			newEventRules, err := s.notificationsClient.ProcessEvent(s.ctx, ev, ruleVersion, eventRuleIds...)
			if errors.Is(err, notifications.ErrRulesOutdated) {
				s.rulesMutex.Lock()
				s.rules = newEventRules
				s.rulesMutex.Unlock()

				eventLogger.Debugw("Re-evaluating rules for event after fetching new rules", zap.String("rules_version", s.rules.Version))

				// Re-evaluate the just fetched rules for the current event.
				goto reevaluateRules
			} else if err != nil {
				eventLogger.Errorw("Cannot submit event to Icinga Notifications",
					zap.String("rules_version", s.rules.Version),
					zap.Any("rules", eventRuleIds),
					zap.Error(err))
				continue
			}

			eventLogger.Debugw("Successfully submitted event to Icinga Notifications", zap.Any("rules", eventRuleIds))
		}
	}
}

// Submit a history entry to be processed by the Source's internal worker loop.
//
// Internally, a buffered channel is used for delivery. So this function should not block. Otherwise, it will abort
// after a second and an error is logged.
func (s *Source) Submit(entity database.Entity) {
	select {
	case <-s.ctx.Done():
		s.logger.Errorw("Source context is done, rejecting submission",
			zap.String("submission", fmt.Sprintf("%+v", entity)),
			zap.Error(s.ctx.Err()))
		return

	case s.inputCh <- entity:
		return

	case <-time.After(time.Second):
		s.logger.Error("Source submission channel is blocking, rejecting submission",
			zap.String("submission", fmt.Sprintf("%+v", entity)))
		return
	}
}
