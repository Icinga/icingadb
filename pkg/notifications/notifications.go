package notifications

import (
	"context"
	"encoding/json"
	"fmt"
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
	"sync"
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

	submissionMutex sync.Mutex // submissionMutex protects not concurrent safe struct fields in Client.Submit, i.e., rulesInfo.
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

// evaluateRulesForObject checks each rule against the Icinga DB SQL database and returns matching rule IDs.
//
// Within the Icinga Notifications relation database, the rules are stored in rule.object_filter as a JSON object
// created by Icinga DB Web. This object contains SQL queries with bindvars for the Icinga DB relational database, to be
// executed with the given host, service and environment IDs. If this query returns at least one row, the rule is
// considered as matching.
//
// Icinga DB Web's JSON structure is described in:
// - https://github.com/Icinga/icingadb-web/pull/1289
// - https://github.com/Icinga/icingadb/pull/998#issuecomment-3442298348
func (client *Client) evaluateRulesForObject(ctx context.Context, hostId, serviceId, environmentId types.Binary) ([]string, error) {
	const icingaDbWebRuleVersion = 1

	type IcingaDbWebQuery struct {
		Query      string   `json:"query"`
		Parameters []string `json:"parameters"`
	}

	type IcingaDbWebRule struct {
		Version int `json:"version"` // expect icingaDbWebRuleVersion
		Queries struct {
			Host    *IcingaDbWebQuery `json:"host,omitempty"`
			Service *IcingaDbWebQuery `json:"service,omitempty"`
		} `json:"queries"`
	}

	outRuleIds := make([]string, 0, len(client.rulesInfo.Rules))

	for id, filterExpr := range client.rulesInfo.Rules {
		if filterExpr == "" {
			outRuleIds = append(outRuleIds, id)
			continue
		}

		var webRule IcingaDbWebRule
		if err := json.Unmarshal([]byte(filterExpr), &webRule); err != nil {
			return nil, errors.Wrap(err, "cannot decode rule filter expression as JSON into struct")
		}
		if version := webRule.Version; version != icingaDbWebRuleVersion {
			return nil, errors.Errorf("decoded rule filter expression .Version is %d, %d expected", version, icingaDbWebRuleVersion)
		}

		var webQuery IcingaDbWebQuery
		if !serviceId.Valid() {
			// Evaluate rule for a host object
			if webRule.Queries.Host == nil {
				continue
			}
			webQuery = *webRule.Queries.Host
		} else {
			// Evaluate rule for a service object
			if webRule.Queries.Service == nil {
				continue
			}
			webQuery = *webRule.Queries.Service
		}

		queryArgs := make([]any, 0, len(webQuery.Parameters))
		for _, param := range webQuery.Parameters {
			switch param {
			case ":host_id":
				queryArgs = append(queryArgs, hostId)
			case ":service_id":
				if !serviceId.Valid() {
					return nil, errors.New("host rule filter expression contains :service_id for replacement")
				}
				queryArgs = append(queryArgs, serviceId)
			case ":environment_id":
				queryArgs = append(queryArgs, environmentId)
			default:
				queryArgs = append(queryArgs, param)
			}
		}

		matches, err := func() (bool, error) {
			rows, err := client.db.QueryContext(ctx, client.db.Rebind(webQuery.Query), queryArgs...)
			if err != nil {
				return false, err
			}
			defer func() { _ = rows.Close() }()

			return rows.Next(), nil
		}()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot fetch rule %q from %q", id, filterExpr)
		} else if !matches {
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
) (*event.Event, *hostServiceInformation, error) {
	info, err := client.fetchHostServiceData(ctx, hostId, serviceId)
	if err != nil {
		return nil, nil, err
	}

	var (
		objectName string
		objectUrl  string
		objectTags map[string]string
	)

	if info.serviceName != "" {
		objectName = info.hostName + "!" + info.serviceName
		objectUrl = "/icingadb/service?name=" + utils.RawUrlEncode(info.serviceName) + "&host.name=" + utils.RawUrlEncode(info.hostName)
		objectTags = map[string]string{
			"host":    info.hostName,
			"service": info.serviceName,
		}
	} else {
		objectName = info.hostName
		objectUrl = "/icingadb/host?name=" + utils.RawUrlEncode(info.hostName)
		objectTags = map[string]string{
			"host": info.hostName,
		}
	}

	return &event.Event{
		Name:      objectName,
		URL:       objectUrl,
		Tags:      objectTags,
		ExtraTags: info.customVars,
	}, info, nil
}

// buildStateHistoryEvent builds a fully initialized event.Event from a state history entry.
//
// The resulted event will have all the necessary information for a state change event, and must
// not be further modified by the caller.
func (client *Client) buildStateHistoryEvent(ctx context.Context, h *v1history.StateHistory) (*event.Event, error) {
	ev, info, err := client.buildCommonEvent(ctx, h.HostId, h.ServiceId)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", h.HostId, h.ServiceId)
	}

	ev.Type = event.TypeState

	if info.serviceName != "" {
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

	var (
		ev          *event.Event
		eventErr    error
		metaHistory v1history.HistoryTableMeta
	)

	// Keep the type switch in sync with the values of SyncKeyStructPtrs below.
	switch h := entity.(type) {
	case *v1history.AcknowledgementHistory:
		ev, eventErr = client.buildAcknowledgementHistoryEvent(client.ctx, h)
		metaHistory = h.HistoryTableMeta

	case *v1history.DowntimeHistoryMeta:
		ev, eventErr = client.buildDowntimeHistoryMetaEvent(client.ctx, h)
		metaHistory = h.HistoryTableMeta

	case *v1history.FlappingHistory:
		ev, eventErr = client.buildFlappingHistoryEvent(client.ctx, h)
		metaHistory = h.HistoryTableMeta

	case *v1history.StateHistory:
		if h.StateType != common.HardState {
			return true
		}
		ev, eventErr = client.buildStateHistoryEvent(client.ctx, h)
		metaHistory = h.HistoryTableMeta

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
		eventRuleIds, err := client.evaluateRulesForObject(
			client.ctx,
			metaHistory.HostId,
			metaHistory.ServiceId,
			metaHistory.EnvironmentId)
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
