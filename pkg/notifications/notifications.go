package notifications

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/internal/config"
	"github.com/icinga/icingadb/pkg/common"
	v1history "github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// IcingaNotificationsEvent represents an event to be processed by Icinga Notifications.
//
// https://github.com/Icinga/icinga-notifications/blob/v0.1.1/internal/event/event.go#L27
type IcingaNotificationsEvent struct {
	Name string            `json:"name"`
	URL  string            `json:"url"`
	Tags map[string]string `json:"tags"`

	Type     string `json:"type"`
	Severity string `json:"severity,omitempty"`
	Username string `json:"username"`
	Message  string `json:"message"`

	Mute       bool   `json:"mute,omitempty"`
	MuteReason string `json:"mute_reason,omitempty"`
}

// MarshalLogObject implements the zapcore.ObjectMarshaler interface.
func (event IcingaNotificationsEvent) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("name", event.Name)
	encoder.AddString("type", event.Type)
	return nil
}

// List of IcingaNotificationsEvent.Type defined by Icinga Notifications.
//
// https://github.com/Icinga/icinga-notifications/blob/v0.1.1/internal/event/event.go#L49-L62
const (
	TypeAcknowledgementCleared = "acknowledgement-cleared"
	TypeAcknowledgementSet     = "acknowledgement-set"
	TypeCustom                 = "custom"
	TypeDowntimeEnd            = "downtime-end"
	TypeDowntimeRemoved        = "downtime-removed"
	TypeDowntimeStart          = "downtime-start"
	TypeFlappingEnd            = "flapping-end"
	TypeFlappingStart          = "flapping-start"
	TypeIncidentAge            = "incident-age"
	TypeMute                   = "mute"
	TypeState                  = "state"
	TypeUnmute                 = "unmute"
)

// Severities inspired by Icinga Notifications.
//
// https://github.com/Icinga/icinga-notifications/blob/v0.1.1/internal/event/severity.go#L9
const (
	SeverityOK      = "ok"
	SeverityDebug   = "debug"
	SeverityInfo    = "info"
	SeverityNotice  = "notice"
	SeverityWarning = "warning"
	SeverityErr     = "err"
	SeverityCrit    = "crit"
	SeverityAlert   = "alert"
	SeverityEmerg   = "emerg"
)

// RuleResp describes a rule response object from Icinga Notifications /event-rules API.
type RuleResp struct {
	Id               int64
	Name             string
	ObjectFilterExpr string
}

// Source is an Icinga Notifications compatible source implementation to push events to Icinga Notifications.
//
// A new Source should be created by the NewNotificationsSource function. New history entries can be submitted by
// calling the Source.Submit method.
type Source struct {
	config.NotificationsConfig

	inputCh chan database.Entity // inputCh is a buffered channel used to submit history entries to the worker.
	db      *database.DB
	logger  *logging.Logger

	rules       map[int64]RuleResp
	ruleVersion string
	rulesMutex  sync.RWMutex

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// ErrRulesOutdated implies that the rule version between Icinga DB and Icinga Notifications mismatches.
var ErrRulesOutdated = fmt.Errorf("rule version is outdated")

// NewNotificationsSource creates a new Source connected to an existing database and logger.
//
// This function starts a worker goroutine in the background which can be stopped by ending the provided context.
func NewNotificationsSource(
	ctx context.Context,
	db *database.DB,
	logger *logging.Logger,
	cfg config.NotificationsConfig,
) *Source {
	ctx, ctxCancel := context.WithCancel(ctx)

	source := &Source{
		NotificationsConfig: cfg,

		inputCh: make(chan database.Entity, 1<<10), // chosen by fair dice roll
		db:      db,
		logger:  logger,

		ctx:       ctx,
		ctxCancel: ctxCancel,
	}
	go source.worker()

	return source
}

// fetchRules from Icinga Notifications /event-rules API endpoint and store both the new rules and the latest rule
// version in the Source struct.
func (s *Source) fetchRules(ctx context.Context, client *http.Client) error {
	apiUrl, err := url.JoinPath(s.ApiBaseUrl, "/event-rules")
	if err != nil {
		return errors.Wrap(err, "cannot join API URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return errors.Wrap(err, "cannot create HTTP request")
	}
	req.SetBasicAuth(s.User, s.Password)

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "cannot GET rules from Icinga Notifications")
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %q (%d) for rules", resp.Status, resp.StatusCode)
	}

	type Response struct {
		Version string
		Rules   map[int64]RuleResp
	}
	var r Response

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&r); err != nil {
		return errors.Wrap(err, "cannot decode rules from Icinga Notifications")
	}

	s.rulesMutex.Lock()
	s.rules = r.Rules
	s.ruleVersion = r.Version
	s.rulesMutex.Unlock()

	return nil
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

	outRuleIds := make([]int64, 0, len(s.rules))

	namedParams := map[string]any{
		"host_id":        hostId,
		"service_id":     serviceId,
		"environment_id": environmentId,
	}

	for _, rule := range s.rules {
		if rule.ObjectFilterExpr == "" {
			outRuleIds = append(outRuleIds, rule.Id)
			continue
		}

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				query := s.db.Rebind(rule.ObjectFilterExpr)
				rows, err := s.db.NamedQueryContext(ctx, query, namedParams)
				if err != nil {
					return err
				}
				defer func() { _ = rows.Close() }()

				if !rows.Next() {
					return sql.ErrNoRows
				}
				return nil
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			retry.Settings{Timeout: retry.DefaultTimeout})

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

// rawurlencode mimics PHP's rawurlencode to be used for parameter encoding.
//
// Icinga Web uses rawurldecode instead of urldecode, which, as its main difference, does not honor the plus char ('+')
// as a valid substitution for space (' '). Unfortunately, Go's url.QueryEscape does this very substitution and
// url.PathEscape does a bit too less and has a misleading name on top.
//
//   - https://www.php.net/manual/en/function.rawurlencode.php
//   - https://github.com/php/php-src/blob/php-8.2.12/ext/standard/url.c#L538
func rawurlencode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// buildCommonEvent creates an event.Event based on Host and (optional) Service attributes to be specified later.
//
// The new Event's Time will be the current timestamp.
//
// The following fields will NOT be populated and might be altered later:
//   - Type
//   - Severity
//   - Username
//   - Message
//   - ID
func (s *Source) buildCommonEvent(host, service string) (*IcingaNotificationsEvent, error) {
	var (
		eventName string
		eventUrl  *url.URL
		eventTags map[string]string
	)

	eventUrl, err := url.Parse(s.IcingaWeb2BaseUrl)
	if err != nil {
		return nil, err
	}

	if service != "" {
		eventName = host + "!" + service

		eventUrl = eventUrl.JoinPath("/icingadb/service")
		eventUrl.RawQuery = "name=" + rawurlencode(service) + "&host.name=" + rawurlencode(host)

		eventTags = map[string]string{
			"host":    host,
			"service": service,
		}
	} else {
		eventName = host

		eventUrl = eventUrl.JoinPath("/icingadb/host")
		eventUrl.RawQuery = "name=" + rawurlencode(host)

		eventTags = map[string]string{
			"host": host,
		}
	}

	return &IcingaNotificationsEvent{
		Name: eventName,
		URL:  eventUrl.String(),
		Tags: eventTags,
	}, nil
}

// buildStateHistoryEvent from a state history entry.
func (s *Source) buildStateHistoryEvent(ctx context.Context, h *v1history.StateHistory) (*IcingaNotificationsEvent, error) {
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	event, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
	}

	event.Type = TypeState

	if serviceName != "" {
		switch h.HardState {
		case 0:
			event.Severity = SeverityOK
		case 1:
			event.Severity = SeverityWarning
		case 2:
			event.Severity = SeverityCrit
		case 3:
			event.Severity = SeverityErr
		default:
			return nil, fmt.Errorf("unexpected service state %d", h.HardState)
		}
	} else {
		switch h.HardState {
		case 0:
			event.Severity = SeverityOK
		case 1:
			event.Severity = SeverityCrit
		default:
			return nil, fmt.Errorf("unexpected host state %d", h.HardState)
		}
	}

	if h.Output.Valid {
		event.Message = h.Output.String
	}
	if h.LongOutput.Valid {
		event.Message += "\n" + h.LongOutput.String
	}

	return event, nil
}

// buildDowntimeHistoryEvent from a downtime history entry.
func (s *Source) buildDowntimeHistoryEvent(ctx context.Context, h *v1history.DowntimeHistory) (*IcingaNotificationsEvent, error) {
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	event, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
	}

	if h.HasBeenCancelled.Valid && h.HasBeenCancelled.Bool {
		event.Type = TypeDowntimeRemoved
		event.Message = "Downtime was cancelled"

		if h.CancelledBy.Valid {
			event.Username = h.CancelledBy.String
		}
	} else if h.EndTime.Time().Compare(time.Now()) <= 0 {
		event.Type = TypeDowntimeEnd
		event.Message = "Downtime expired"
	} else {
		event.Type = TypeDowntimeStart
		event.Username = h.Author
		event.Message = h.Comment
		event.Mute = true
		event.MuteReason = "Checkable is in downtime"
	}

	return event, nil
}

// buildFlappingHistoryEvent from a flapping history entry.
func (s *Source) buildFlappingHistoryEvent(ctx context.Context, h *v1history.FlappingHistory) (*IcingaNotificationsEvent, error) {
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	event, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
	}

	if h.PercentStateChangeEnd.Valid {
		event.Type = TypeFlappingEnd
		event.Message = fmt.Sprintf(
			"Checkable stopped flapping (Current flapping value %.2f%% < low threshold %.2f%%)",
			h.PercentStateChangeEnd.Float64, h.FlappingThresholdLow)
	} else if h.PercentStateChangeStart.Valid {
		event.Type = TypeFlappingStart
		event.Message = fmt.Sprintf(
			"Checkable started flapping (Current flapping value %.2f%% > high threshold %.2f%%)",
			h.PercentStateChangeStart.Float64, h.FlappingThresholdHigh)
		event.Mute = true
		event.MuteReason = "Checkable is flapping"
	} else {
		return nil, errors.New("flapping history entry has neither percent_state_change_start nor percent_state_change_end")
	}

	return event, nil
}

// buildAcknowledgementHistoryEvent from an acknowledgement history entry.
func (s *Source) buildAcknowledgementHistoryEvent(ctx context.Context, h *v1history.AcknowledgementHistory) (*IcingaNotificationsEvent, error) {
	hostName, serviceName, err := s.fetchHostServiceName(ctx, h.HostId, h.ServiceId, h.EnvironmentId)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch host/service information")
	}

	event, err := s.buildCommonEvent(hostName, serviceName)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build event for %q,%q", hostName, serviceName)
	}

	if !h.ClearTime.Time().IsZero() {
		event.Type = TypeAcknowledgementCleared
		event.Message = "Checkable was cleared"

		if h.ClearedBy.Valid {
			event.Username = h.ClearedBy.String
		}
	} else if !h.SetTime.Time().IsZero() {
		event.Type = TypeAcknowledgementSet

		if h.Comment.Valid {
			event.Message = h.Comment.String
		} else {
			event.Message = "Checkable was acknowledged"
		}

		if h.Author.Valid {
			event.Username = h.Author.String
		}
	} else {
		return nil, errors.New("acknowledgment history entry has neither a set_time nor a clear_time")
	}

	return event, nil
}

// submitEvent to the Icinga Notifications /process-event API endpoint.
//
// The event will be passed together with the Source.ruleVersion and all evaluated ruleIds to the endpoint. Even if no
// rules were evaluated, this method should be called. Thus, Icinga Notifications can dismiss the event, but Icinga DB
// would still be informed in case of a rule change. Otherwise, events might be dropped here which are now required.
//
// This method may return an ErrRulesOutdated error, implying that the Source.ruleVersion mismatches the version stored
// at Icinga Notifications. In this case, the rules must be refetched and the event requires another evaluation.
func (s *Source) submitEvent(ctx context.Context, client *http.Client, event *IcingaNotificationsEvent, ruleIds []int64) error {
	jsonBody, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "cannot encode event to JSON")
	}

	apiUrl, err := url.JoinPath(s.ApiBaseUrl, "/process-event")
	if err != nil {
		return errors.Wrap(err, "cannot join API URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return errors.Wrap(err, "cannot create HTTP request")
	}
	req.SetBasicAuth(s.User, s.Password)

	s.rulesMutex.RLock()
	ruleVersion := s.ruleVersion
	s.rulesMutex.RUnlock()

	ruleIdsStrArr := make([]string, 0, len(ruleIds))
	for _, idInt := range ruleIds {
		ruleIdsStrArr = append(ruleIdsStrArr, fmt.Sprintf("%d", idInt))
	}
	ruleIdsStr := strings.Join(ruleIdsStrArr, ",")

	req.Header.Set("X-Rule-Ids", ruleIdsStr)
	req.Header.Set("X-Rule-Version", ruleVersion)

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "cannot POST HTTP request to Icinga Notifications")
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Refetching rules required.
	if resp.StatusCode == http.StatusFailedDependency {
		return ErrRulesOutdated
	}

	// 200s are acceptable.
	if 200 <= resp.StatusCode && resp.StatusCode <= 299 {
		return nil
	}

	// Ignoring superfluous state change.
	if resp.StatusCode == http.StatusNotAcceptable {
		return nil
	}

	var buff bytes.Buffer
	_, _ = io.Copy(&buff, &io.LimitedReader{
		R: resp.Body,
		N: 1 << 20, // Limit the error message's length against memory exhaustion
	})
	return fmt.Errorf("unexpected response from Icinga Notificatios, status %q (%d): %q",
		resp.Status, resp.StatusCode, strings.TrimSpace(buff.String()))
}

// worker is the background worker launched by NewNotificationsSource.
func (s *Source) worker() {
	defer s.ctxCancel()

	client := &http.Client{}

	if err := retry.WithBackoff(
		s.ctx,
		func(ctx context.Context) error { return s.fetchRules(s.ctx, client) },
		func(_ error) bool { return true }, // For the moment, retry every potential error.
		backoff.DefaultBackoff,
		retry.Settings{
			Timeout: retry.DefaultTimeout,
			OnRetryableError: func(elapsed time.Duration, attempt uint64, err, lastErr error) {
				s.logger.Errorw("Cannot fetch rules from Icinga Notifications",
					zap.Duration("elapsed", elapsed),
					zap.Uint64("attempt", attempt),
					zap.Error(err))
			},
			OnSuccess: func(_ time.Duration, attempt uint64, _ error) {
				s.logger.Infow("Fetched rules from Icinga Notifications", zap.Uint64("attempt", attempt))
			},
		},
	); err != nil {
		s.logger.Fatalw("Cannot fetch rules from Icinga Notifications", zap.Error(err))
	}

	for {
		select {
		case <-s.ctx.Done():
			return

		case entity := <-s.inputCh:
			var (
				event       *IcingaNotificationsEvent
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

				event, eventErr = s.buildStateHistoryEvent(s.ctx, h)
				metaHistory = h.HistoryTableMeta

			case *v1history.DowntimeHistory:
				event, eventErr = s.buildDowntimeHistoryEvent(s.ctx, h)
				metaHistory = h.HistoryTableMeta

			case *v1history.CommentHistory:
				// Ignore for the moment.
				continue

			case *v1history.FlappingHistory:
				event, eventErr = s.buildFlappingHistoryEvent(s.ctx, h)
				metaHistory = h.HistoryTableMeta

			case *v1history.AcknowledgementHistory:
				event, eventErr = s.buildAcknowledgementHistoryEvent(s.ctx, h)
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
			if event == nil {
				s.logger.Error("No event was fetched, but no error was reported. This REALLY SHOULD NOT happen.")
				continue
			}

			eventLogger := s.logger.With(zap.Object("event", event))

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

			err = s.submitEvent(s.ctx, client, event, eventRuleIds)
			if errors.Is(err, ErrRulesOutdated) {
				s.logger.Info("Icinga Notification rules were updated, triggering resync")

				if err := s.fetchRules(s.ctx, client); err != nil {
					s.logger.Errorw("Cannot fetch rules from Icinga Notifications", zap.Error(err))
				}
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
