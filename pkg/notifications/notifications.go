package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/retry"
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
}

// NewNotificationsSource creates a new Source connected to an existing database and logger.
//
// This function starts a worker goroutine in the background which can be stopped by ending the provided context.
func NewNotificationsSource(
	ctx context.Context,
	db *database.DB,
	logger *logging.Logger,
	cfg notifications.Config,
) *Source {
	ctx, ctxCancel := context.WithCancel(ctx)

	source := &Source{
		Config: cfg,

		inputCh: make(chan database.Entity, 1<<10), // chosen by fair dice roll
		db:      db,
		logger:  logger,

		rules: &notifications.SourceRulesInfo{Version: notifications.EmptyRulesVersion},

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
// For the queries, the following mapping is performed:
//   - :host_id <- hostId
//   - :service_id <- serviceId
//   - :environment_id <- environmentId
//
// This allows a query like the following:
//
//	> select * from host where id = :host_id and environment_id = :environment_id and name like 'prefix_%'
func (s *Source) evaluateRulesForObject(ctx context.Context, hostId, serviceId, environmentId types.Binary) ([]int64, error) {
	s.rulesMutex.RLock()
	defer s.rulesMutex.RUnlock()

	outRuleIds := make([]int64, 0, len(s.rules.Rules))

	namedParams := map[string]any{
		"host_id":        hostId,
		"service_id":     serviceId,
		"environment_id": environmentId,
	}

	for rule := range s.rules.Iter() {
		if rule.ObjectFilterExpr == "" {
			outRuleIds = append(outRuleIds, rule.Id)
			continue
		}

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				// The raw SQL query in the database is URL-encoded (mostly the space character is replaced by %20).
				// So, we need to unescape it before passing it to the database.
				query, err := url.QueryUnescape(rule.ObjectFilterExpr)
				if err != nil {
					return errors.Wrapf(err, "cannot unescape rule %d object filter expression %q", rule.Id, rule.ObjectFilterExpr)
				}
				rows, err := s.db.NamedQueryContext(ctx, s.db.Rebind(query), namedParams)
				if err != nil {
					return err
				}
				defer func() { _ = rows.Close() }()

				if !rows.Next() {
					return sql.ErrNoRows
				}
				return nil
			},
			func(_ error) bool { return false }, // Never retry an error, otherwise we'll block the worker unnecessarily.
			backoff.DefaultBackoff,
			s.db.GetDefaultRetrySettings(),
		)

		if err == nil {
			outRuleIds = append(outRuleIds, rule.Id)
		} else if errors.Is(err, sql.ErrNoRows) {
			continue
		} else {
			return nil, errors.Wrapf(err, "cannot fetch rule %d from %q", rule.Id, rule.ObjectFilterExpr)
		}
	}

	return outRuleIds[:len(outRuleIds):len(outRuleIds)], nil
}

// fetchHostServiceName for a host ID and a potential service ID from the Icinga DB relational database.
func (s *Source) fetchHostServiceName(ctx context.Context, hostId, serviceId, envId types.Binary) (host, service string, err error) {
	err = retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			queryHost := s.db.Rebind("SELECT name FROM host WHERE id = ? AND environment_id = ?")
			err := s.db.QueryRowxContext(ctx, queryHost, hostId, envId).Scan(&host)
			if err != nil {
				return errors.Wrap(err, "cannot select host")
			}

			if serviceId != nil {
				queryService := s.db.Rebind("SELECT name FROM service WHERE id = ? AND environment_id = ?")
				err := s.db.QueryRowxContext(ctx, queryService, serviceId, envId).Scan(&service)
				if err != nil {
					return errors.Wrap(err, "cannot select service")
				}
			}

			return nil
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		retry.Settings{Timeout: retry.DefaultTimeout})
	return
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service names.
//
// This function is used by all event builders to create a common event structure that includes the host and service
// names, the absolute URL to the Icinga Web 2 Icinga DB page for the host or service, and the tags for the event.
// Any event type-specific information (like severity, message, etc.) is added by the specific event builders.
func (s *Source) buildCommonEvent(host, service string) (*event.Event, error) {
	var (
		eventName string
		eventUrl  *url.URL
		eventTags map[string]string
	)

	if service != "" {
		eventName = host + "!" + service

		eventUrl = s.notificationsClient.JoinIcingaWeb2Path("/icingadb/service")
		eventUrl.RawQuery = "name=" + utils.RawUrlEncode(service) + "&host.name=" + utils.RawUrlEncode(host)

		eventTags = map[string]string{
			"host":    host,
			"service": service,
		}
	} else {
		eventName = host

		eventUrl = s.notificationsClient.JoinIcingaWeb2Path("/icingadb/host")
		eventUrl.RawQuery = "name=" + utils.RawUrlEncode(host)

		eventTags = map[string]string{
			"host": host,
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
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	ev, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
	}

	ev.Type = event.TypeState

	if serviceName != "" {
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
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	ev, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
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
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	ev, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
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
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	ev, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
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

		case entity := <-s.inputCh:
			var (
				ev          *event.Event
				eventErr    error
				metaHistory v1history.HistoryTableMeta
			)

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
				metaHistory = h.HistoryTableMeta

			case *v1history.DowntimeHistory:
				ev, eventErr = s.buildDowntimeHistoryEvent(s.ctx, h)
				metaHistory = h.HistoryTableMeta

			case *v1history.CommentHistory:
				// Ignore for the moment.
				continue

			case *v1history.FlappingHistory:
				ev, eventErr = s.buildFlappingHistoryEvent(s.ctx, h)
				metaHistory = h.HistoryTableMeta

			case *v1history.AcknowledgementHistory:
				ev, eventErr = s.buildAcknowledgementHistoryEvent(s.ctx, h)
				metaHistory = h.HistoryTableMeta

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

			eventRuleIds, err := s.evaluateRulesForObject(
				s.ctx,
				metaHistory.HostId,
				metaHistory.ServiceId,
				metaHistory.EnvironmentId)
			if err != nil {
				eventLogger.Errorw("Cannot evaluate rules for event", zap.Error(err))
				continue
			}

			eventLogger = eventLogger.With(zap.Any("rules", eventRuleIds))

			s.rulesMutex.RLock()
			ruleVersion := s.rules.Version
			s.rulesMutex.RUnlock()

			newEventRules, err := s.notificationsClient.ProcessEvent(s.ctx, ev, ruleVersion, eventRuleIds...)
			if errors.Is(err, notifications.ErrRulesOutdated) {
				s.rulesMutex.Lock()
				s.rules = newEventRules
				s.rulesMutex.Unlock()

				go s.Submit(entity)

				continue
			} else if err != nil {
				eventLogger.Errorw("Cannot submit event to Icinga Notifications", zap.Error(err))
				continue
			}

			eventLogger.Info("Submitted event to Icinga Notifications")
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
