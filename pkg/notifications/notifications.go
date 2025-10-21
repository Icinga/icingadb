package notifications

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	v1history "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Client is an Icinga Notifications compatible client implementation to push events to Icinga Notifications.
//
// A new Client should be created by the NewNotificationsClient function. New history entries can be submitted by
// calling the Client.Submit method.
type Client struct {
	source.Config

	db     *database.DB
	logger *logging.Logger

	rulesInfo *source.RulesInfo // rulesInfo holds the latest rulesInfo fetched from Icinga Notifications.

	ctx context.Context

	notificationsClient *source.Client // The Icinga Notifications client used to interact with the API.
	redisClient         *redis.Client  // redisClient is the Redis client used to fetch host and service names for events.

	submissionMutex sync.Mutex
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

		rulesInfo: &source.RulesInfo{},

		notificationsClient: notificationsClient,
		redisClient:         rc,
	}, nil
}

// evaluateRulesForObject returns the rule IDs for each matching query.
//
// At the moment, each rule filter expression is executed as a SQL query after the parameters are being bound. If the
// query returns at least one line, the rule will match. Rules with an empty filter expression are a special case and
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
func (client *Client) evaluateRulesForObject(ctx context.Context, entity database.Entity) ([]string, error) {
	outRuleIds := make([]string, 0, len(client.rulesInfo.Rules))

	for id, filterExpr := range client.rulesInfo.Rules {
		if filterExpr == "" {
			outRuleIds = append(outRuleIds, id)
			continue
		}

		evaluates, err := func() (bool, error) {
			// The raw SQL query in the database is URL-encoded (mostly the space character is replaced by %20).
			// So, we need to unescape it before passing it to the database.
			query, err := url.QueryUnescape(filterExpr)
			if err != nil {
				return false, errors.Wrapf(err, "cannot unescape rule %q object filter expression %q", id, filterExpr)
			}
			rows, err := client.db.NamedQueryContext(ctx, client.db.Rebind(query), entity)
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
			return nil, errors.Wrapf(err, "cannot fetch rule %q from %q", id, filterExpr)
		} else if !evaluates {
			continue
		}
		outRuleIds = append(outRuleIds, id)
	}

	return outRuleIds, nil
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service IDs.
//
// This function is used by all event builders to create a common event structure that includes the host and service
// names, an Icinga DB Web reference, and the tags for the event.
// Any event type-specific information (like severity, message, etc.) is added by the specific event builders.
func (client *Client) buildCommonEvent(
	ctx context.Context,
	hostId, serviceId types.Binary,
) (*event.Event, *redisLookupResult, error) {
	rlr, err := client.fetchHostServiceName(ctx, hostId, serviceId)
	if err != nil {
		return nil, nil, err
	}

	var (
		objectName string
		objectUrl  string
		objectTags map[string]string
	)

	if rlr.ServiceName != "" {
		objectName = rlr.HostName + "!" + rlr.ServiceName
		objectUrl = "/icingadb/service?name=" + utils.RawUrlEncode(rlr.ServiceName) + "&host.name=" + utils.RawUrlEncode(rlr.HostName)
		objectTags = map[string]string{
			"host":    rlr.HostName,
			"service": rlr.ServiceName,
		}
	} else {
		objectName = rlr.HostName
		objectUrl = "/icingadb/host?name=" + utils.RawUrlEncode(rlr.HostName)
		objectTags = map[string]string{
			"host": rlr.HostName,
		}
	}

	return &event.Event{
		Name: objectName,
		URL:  objectUrl,
		Tags: objectTags,
	}, rlr, nil
}

// buildStateHistoryEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (client *Client) buildStateHistoryEvent(ctx context.Context, h *v1history.StateHistory) (*event.Event, error) {
	ev, rlr, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
	}

	ev.Type = event.TypeState

	if rlr.ServiceName != "" {
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
func (client *Client) buildDowntimeHistoryMetaEvent(ctx context.Context, h *v1history.DowntimeHistoryMeta) (*event.Event, error) {
	ev, _, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
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
func (client *Client) buildFlappingHistoryEvent(ctx context.Context, h *v1history.FlappingHistory) (*event.Event, error) {
	ev, _, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
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
func (client *Client) buildAcknowledgementHistoryEvent(ctx context.Context, h *v1history.AcknowledgementHistory) (*event.Event, error) {
	ev, _, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
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

// Submit this [database.Entity] to the Icinga Notifications API.
//
// Based on the entity's type, a different kind of event will be constructed. The event will be sent to the API in a
// blocking fashion.
//
// Returns true if this entity was processed or cannot be processed any further. Returns false if this entity should be
// retried later.
//
// This method usees the Client's logger.
func (client *Client) Submit(entity database.Entity) bool {
	if client.ctx.Err() != nil {
		client.logger.Errorw("Cannot process submitted entity as client context is done", zap.Error(client.ctx.Err()))
		return true
	}

	var ev *event.Event
	var eventErr error

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

	eventLogger := client.logger.With(zap.Object(
		"event",
		zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
			encoder.AddString("name", ev.Name)
			encoder.AddString("type", ev.Type.String())
			return nil
		}),
	))

	// The following code accesses Client.rulesInfo.
	client.submissionMutex.Lock()
	defer client.submissionMutex.Unlock()

	// This loop allows resubmitting an event if the rules have changed. The first try would be the rule update, the
	// second try would be the resubmit, and the third try would be for bad luck, e.g., when a second rule update just
	// crept in between. If there are three subsequent rule updates, something is wrong.
	for try := 0; try < 3; try++ {
		eventRuleIds, err := client.evaluateRulesForObject(client.ctx, entity)
		if err != nil {
			// While returning false would be more correct, this would result in never being able to refetch new rule
			// versions. Consider an invalid object filter expression, which is now impossible to get rid of.
			eventLogger.Errorw("Cannot evaluate rules for event, assuming no rule matched", zap.Error(err))
			eventRuleIds = []string{}
		}

		ev.RulesVersion = client.rulesInfo.Version
		ev.RuleIds = eventRuleIds

		newEventRules, err := client.notificationsClient.ProcessEvent(client.ctx, ev)
		if errors.Is(err, source.ErrRulesOutdated) {
			eventLogger.Infow("Received a rule update from Icinga Notification, resubmitting event",
				zap.String("old_rules_version", client.rulesInfo.Version),
				zap.String("new_rules_version", newEventRules.Version))

			client.rulesInfo = newEventRules

			continue
		} else if err != nil {
			eventLogger.Errorw("Cannot submit event to Icinga Notifications, will be retried",
				zap.String("rules_version", client.rulesInfo.Version),
				zap.Any("rules", eventRuleIds),
				zap.Error(err))
			return false
		}

		eventLogger.Debugw("Successfully submitted event to Icinga Notifications", zap.Any("rules", eventRuleIds))
		return true
	}

	eventLogger.Error("Received three rule updates from Icinga Notifications in a row, event will be retried")
	return false
}

var SyncKeyStructPtrs = map[string]any{
	history.SyncPipelineAcknowledgement: (*v1history.AcknowledgementHistory)(nil),
	history.SyncPipelineDowntime:        (*v1history.DowntimeHistoryMeta)(nil),
	history.SyncPipelineFlapping:        (*v1history.FlappingHistory)(nil),
	history.SyncPipelineState:           (*v1history.StateHistory)(nil),
}
