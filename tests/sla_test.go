package icingadb_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/icinga/icinga-testing/utils"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"math"
	"net/http"
	"testing"
	"time"
)

func TestSla(t *testing.T) {
	m := it.MysqlDatabaseT(t)
	m.ImportIcingaDbSchema()

	r := it.RedisServerT(t)
	i := it.Icinga2NodeT(t, "master")
	i.EnableIcingaDb(r)
	err := i.Reload()
	require.NoError(t, err, "icinga2 should reload without error")
	it.IcingaDbInstanceT(t, r, m)

	client := i.ApiClient()

	t.Run("StateEvents", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_passive_checks": true,
				"check_command":         "dummy",
				"max_check_attempts":    3,
			},
		})

		type StateChange struct {
			Time  float64
			State int
		}

		var stateChanges []StateChange

		processCheckResult := func(exitStatus int, isHard bool) *ObjectsHostsResponse {
			time.Sleep(10 * time.Millisecond) // ensure there is a bit of difference in ms resolution

			output := utils.UniqueName(t, "output")
			data := ActionsProcessCheckResultRequest{
				Type:         "Host",
				Filter:       fmt.Sprintf(`host.name==%q`, hostname),
				ExitStatus:   exitStatus,
				PluginOutput: output,
			}
			dataJson, err := json.Marshal(data)
			require.NoError(t, err, "marshal request")
			response, err := client.PostJson("/v1/actions/process-check-result", bytes.NewBuffer(dataJson))
			require.NoError(t, err, "process-check-result")
			require.Equal(t, 200, response.StatusCode, "process-check-result")

			response, err = client.GetJson("/v1/objects/hosts/" + hostname)
			require.NoError(t, err, "get host: request")
			require.Equal(t, 200, response.StatusCode, "get host: request")

			var hosts ObjectsHostsResponse
			err = json.NewDecoder(response.Body).Decode(&hosts)
			require.NoError(t, err, "get host: parse response")

			require.Equal(t, 1, len(hosts.Results), "there must be one host in the response")
			host := hosts.Results[0]
			require.Equal(t, output, host.Attrs.LastCheckResult.Output,
				"last check result should be visible in host object")
			require.Equal(t, exitStatus, host.Attrs.State, "soft state should match check result")

			if isHard {
				require.Equal(t, exitStatus, host.Attrs.LastHardState, "hard state should match check result")
				if len(stateChanges) > 0 {
					require.Greater(t, host.Attrs.LastHardStateChange, stateChanges[len(stateChanges)-1].Time,
						"last_hard_state_change_time of host should have changed")
				}
				stateChanges = append(stateChanges, StateChange{
					Time:  host.Attrs.LastHardStateChange,
					State: exitStatus,
				})
			} else {
				require.NotEmpty(t, stateChanges, "there should be a hard state change prior to a soft one")
				require.Equal(t, stateChanges[len(stateChanges)-1].Time, host.Attrs.LastHardStateChange,
					"check result should not lead to a hard state change, i.e. last_hard_state_change should not change")
			}

			return &hosts
		}

		processCheckResult(0, true)  // hard (UNKNOWN -> UP)
		processCheckResult(1, false) // soft
		processCheckResult(1, false) // soft
		processCheckResult(1, true)  // hard (UP -> DOWN)
		processCheckResult(1, false) // hard
		processCheckResult(0, true)  // hard (DOWN -> UP)
		processCheckResult(0, false) // hard

		assert.Equal(t, 3, len(stateChanges), "there should be three hard state changes")

		db, err := sqlx.Connect("mysql", m.DSN())
		require.NoError(t, err, "connecting to mysql")
		defer func() { _ = db.Close() }()

		type Row struct {
			Time  int64 `db:"event_time"`
			State int   `db:"hard_state"`
		}

		eventually.Assert(t, func(t require.TestingT) {
			var rows []Row
			err = db.Select(&rows, db.Rebind("SELECT s.event_time, s.hard_state FROM sla_history_state s "+
				"JOIN host ON host.id = s.host_id WHERE host.name = ? ORDER BY event_time ASC"), hostname)
			require.NoError(t, err, "select sla_history_state")

			assert.Equal(t, len(stateChanges), len(rows), "number of sla_history_state entries")

			for i := range rows {
				assert.WithinDuration(t, time.UnixMilli(int64(stateChanges[i].Time*1000)), time.UnixMilli(rows[i].Time),
					time.Millisecond, "event time should match state change time")
				assert.Equal(t, stateChanges[i].State, rows[i].State, "hard state should match")
			}
		}, 5*time.Second, 200*time.Millisecond)

		redis := r.Open()
		defer func() { _ = redis.Close() }()

		logger := it.Logger(t)

		logger.Debug("redis state history", zap.Bool("before", true))
		eventually.Assert(t, func(t require.TestingT) {
			result, err := redis.XRange(context.Background(), "icinga:history:stream:state", "-", "+").Result()
			require.NoError(t, err, "reading state history stream should not fail")
			logger.Debug("redis state history", zap.Any("values", result))
			assert.Empty(t, result, "redis state history stream should be drained")
		}, 5*time.Second, 10*time.Millisecond)
		logger.Debug("redis state history", zap.Bool("after", true))
	})

	t.Run("DowntimeEvents", func(t *testing.T) {
		t.Parallel()

		type Options struct {
			Fixed  bool // Whether to schedule a fixed or flexible downtime.
			Cancel bool // Whether to cancel the downtime or let it expire.
		}

		downtimeTest := func(t *testing.T, o Options) {
			hostname := utils.UniqueName(t, "host")
			client.CreateHost(t, hostname, map[string]interface{}{
				"attrs": map[string]interface{}{
					"enable_active_checks":  false,
					"enable_passive_checks": true,
					"check_command":         "dummy",
					"max_check_attempts":    1,
				},
			})

			processCheckResult := func(status int) time.Time {
				output := utils.RandomString(8)
				reqBody, err := json.Marshal(ActionsProcessCheckResultRequest{
					Type:         "Host",
					Filter:       fmt.Sprintf(`host.name==%q`, hostname),
					ExitStatus:   status,
					PluginOutput: output,
				})
				require.NoError(t, err, "marshal request")
				response, err := client.PostJson("/v1/actions/process-check-result", bytes.NewBuffer(reqBody))
				require.NoError(t, err, "process-check-result")
				require.Equal(t, 200, response.StatusCode, "process-check-result")

				response, err = client.GetJson("/v1/objects/hosts/" + hostname)
				require.NoError(t, err, "get host: request")
				require.Equal(t, 200, response.StatusCode, "get host: request")

				var hosts ObjectsHostsResponse
				err = json.NewDecoder(response.Body).Decode(&hosts)
				require.NoError(t, err, "get host: parse response")

				require.Equal(t, 1, len(hosts.Results), "there must be one host in the response")
				host := hosts.Results[0]
				require.Equal(t, output, host.Attrs.LastCheckResult.Output,
					"last check result should be visible in host object")
				require.Equal(t, 1, host.Attrs.StateType, "host should be in hard state")
				require.Equal(t, status, host.Attrs.State, "state should match check result")

				sec, nsec := math.Modf(host.Attrs.LastCheckResult.ExecutionEnd)
				return time.Unix(int64(sec), int64(nsec*1e9))
			}

			// Ensure that host is in UP state.
			processCheckResult(0)

			refTime := time.Now().Truncate(time.Second)
			// Schedule the downtime start in the past so that we would notice if Icinga 2/DB would
			// use the current time somewhere where we expect the scheduled start time.
			downtimeStart := refTime.Add(-1 * time.Hour)
			var downtimeEnd time.Time
			if o.Cancel || !o.Fixed {
				// Downtimes we will cancel can expire long in the future as we don't have to wait for it.
				// Same for flexible downtimes as for these, we don't have to wait until the scheduled end but only
				// for their duration.
				downtimeEnd = refTime.Add(1 * time.Hour)
			} else {
				// Let all other downtimes expire soon (fixed downtimes where we wait for expiry).
				downtimeEnd = refTime.Add(5 * time.Second)
			}

			var duration time.Duration
			if !o.Fixed {
				duration = 10 * time.Second
			}
			req, err := json.Marshal(ActionsScheduleDowntimeRequest{
				Type:      "Host",
				Filter:    fmt.Sprintf(`host.name==%q`, hostname),
				StartTime: downtimeStart.Unix(),
				EndTime:   downtimeEnd.Unix(),
				Fixed:     o.Fixed,
				Duration:  duration.Seconds(),
				Author:    utils.RandomString(8),
				Comment:   utils.RandomString(8),
			})
			require.NoError(t, err, "marshal request")
			response, err := client.PostJson("/v1/actions/schedule-downtime", bytes.NewBuffer(req))
			require.NoError(t, err, "schedule-downtime")
			require.Equal(t, 200, response.StatusCode, "schedule-downtime")

			var scheduleResponse ActionsScheduleDowntimeResponse
			err = json.NewDecoder(response.Body).Decode(&scheduleResponse)
			require.NoError(t, err, "decode schedule-downtime response")
			require.Equal(t, 1, len(scheduleResponse.Results), "schedule-downtime should return 1 result")
			require.Equal(t, http.StatusOK, scheduleResponse.Results[0].Code, "schedule-downtime should return 1 result")
			downtimeName := scheduleResponse.Results[0].Name

			type Row struct {
				Start int64 `db:"downtime_start"`
				End   int64 `db:"downtime_end"`
			}

			db, err := sqlx.Connect("mysql", m.DSN())
			require.NoError(t, err, "connecting to mysql")
			defer func() { _ = db.Close() }()

			if !o.Fixed {
				// Give Icinga 2 and Icinga DB some time that if they would generate an SLA history event in error,
				// they have a chance to do so before we check for its absence.
				time.Sleep(10 * time.Second)

				var count int
				err = db.Get(&count, db.Rebind("SELECT COUNT(*) FROM sla_history_downtime s "+
					"JOIN host ON host.id = s.host_id WHERE host.name = ?"), hostname)
				require.NoError(t, err, "select sla_history_state")
				assert.Zero(t, count, "there should be no event in sla_history_downtime when scheduling a flexible downtime on an UP host")
			}

			// Bring host into DOWN state.
			criticalTime := processCheckResult(1)

			eventually.Assert(t, func(t require.TestingT) {
				var rows []Row
				err = db.Select(&rows, db.Rebind("SELECT s.downtime_start, s.downtime_end FROM sla_history_downtime s "+
					"JOIN host ON host.id = s.host_id WHERE host.name = ?"), hostname)
				require.NoError(t, err, "select sla_history_state")

				require.Equal(t, 1, len(rows), "there should be exactly one sla_history_downtime row")
				if o.Fixed {
					assert.Equal(t, downtimeStart, time.UnixMilli(rows[0].Start),
						"downtime_start should match scheduled start time")
					assert.Equal(t, downtimeEnd, time.UnixMilli(rows[0].End),
						"downtime_end should match scheduled end time")
				} else {
					assert.WithinDuration(t, criticalTime, time.UnixMilli(rows[0].Start), time.Second,
						"downtime_start should match time of host state change")
					assert.Equal(t, duration, time.UnixMilli(rows[0].End).Sub(time.UnixMilli(rows[0].Start)),
						"downtime_end - downtime_start duration should match scheduled duration")
				}
			}, 5*time.Second, 200*time.Millisecond)

			redis := r.Open()
			defer func() { _ = redis.Close() }()

			eventually.Assert(t, func(t require.TestingT) {
				result, err := redis.XRange(context.Background(), "icinga:history:stream:downtime", "-", "+").Result()
				require.NoError(t, err, "reading downtime history stream should not fail")
				assert.Empty(t, result, "redis downtime history stream should be drained")
			}, 5*time.Second, 10*time.Millisecond)

			if o.Cancel {
				req, err = json.Marshal(ActionsRemoveDowntimeRequest{
					Downtime: downtimeName,
				})
				require.NoError(t, err, "marshal remove-downtime request")
				response, err = client.PostJson("/v1/actions/remove-downtime", bytes.NewBuffer(req))
				require.NoError(t, err, "remove-downtime")
				require.Equal(t, 200, response.StatusCode, "remove-downtime")
			}

			downtimeCancel := time.Now()

			if !o.Cancel {
				// Wait for downtime to expire + a few extra seconds. The row should not be updated, give
				// enough time to have a chance catching if Icinga DB updates it nonetheless.
				if !o.Fixed {
					time.Sleep(duration + 5*time.Second)
				} else {
					d := time.Until(downtimeEnd) + 5*time.Second
					require.Less(t, d, time.Minute, "bug in tests: don't wait too long")
					time.Sleep(d)
				}
			}

			eventually.Assert(t, func(t require.TestingT) {
				var rows []Row
				err = db.Select(&rows, db.Rebind("SELECT s.downtime_start, s.downtime_end FROM sla_history_downtime s "+
					"JOIN host ON host.id = s.host_id WHERE host.name = ?"), hostname)
				require.NoError(t, err, "select sla_history_state")

				require.Equal(t, 1, len(rows), "there should be exactly one sla_history_downtime row")
				if o.Fixed {
					assert.Equal(t, downtimeStart, time.UnixMilli(rows[0].Start),
						"downtime_start should match scheduled start")
				} else {
					assert.WithinDuration(t, criticalTime, time.UnixMilli(rows[0].Start), time.Second,
						"downtime_start should match critical time")
				}
				if o.Cancel {
					// Allow more delta for the end time after cancel as we did not choose the exact time.
					assert.WithinDuration(t, downtimeCancel, time.UnixMilli(rows[0].End), time.Second,
						"downtime_end should match cancel time")
				} else if o.Fixed {
					assert.Equal(t, downtimeEnd, time.UnixMilli(rows[0].End),
						"downtime_start should match scheduled end")
				} else {
					assert.Equal(t, duration, time.UnixMilli(rows[0].End).Sub(time.UnixMilli(rows[0].Start)),
						"downtime_end - downtime_start duration should match scheduled duration")
				}
			}, 5*time.Second, 200*time.Millisecond)

			eventually.Assert(t, func(t require.TestingT) {
				result, err := redis.XRange(context.Background(), "icinga:history:stream:downtime", "-", "+").Result()
				require.NoError(t, err, "reading downtime history stream should not fail")
				assert.Empty(t, result, "redis downtime history stream should be drained")
			}, 5*time.Second, 10*time.Millisecond)
		}

		t.Run("Fixed", func(t *testing.T) {
			t.Parallel()

			t.Run("Cancel", func(t *testing.T) {
				t.Parallel()
				downtimeTest(t, Options{Fixed: true, Cancel: true})
			})

			t.Run("Expire", func(t *testing.T) {
				t.Parallel()
				downtimeTest(t, Options{Fixed: true, Cancel: false})
			})
		})

		t.Run("Flexible", func(t *testing.T) {
			t.Parallel()

			t.Run("Cancel", func(t *testing.T) {
				t.Parallel()
				downtimeTest(t, Options{Fixed: false, Cancel: true})
			})

			t.Run("Expire", func(t *testing.T) {
				t.Parallel()
				downtimeTest(t, Options{Fixed: false, Cancel: false})
			})
		})
	})
}
