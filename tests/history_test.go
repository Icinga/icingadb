package icingadb_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icinga-testing/utils"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"testing"
	"time"
)

func TestHistory(t *testing.T) {
	m := it.MysqlDatabaseT(t)
	m.ImportIcingaDbSchema()

	r := it.RedisServerT(t)
	i := it.Icinga2NodeT(t, "master")
	i.EnableIcingaDb(r)
	err := i.Reload()
	require.NoError(t, err, "icinga2 should reload without error")
	it.IcingaDbInstanceT(t, r, m)

	client := i.ApiClient()

	db, err := sqlx.Connect("mysql", m.DSN())
	require.NoError(t, err, "connecting to mysql")
	t.Cleanup(func() { _ = db.Close() })

	redisClient := r.Open()
	t.Cleanup(func() { _ = redisClient.Close() })

	t.Run("Acknowledgement", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_passive_checks": true,
				"check_command":         "dummy",
				"max_check_attempts":    1,
			},
		})

		processCheckResult(t, client, hostname, 1)

		author := utils.RandomString(8)
		comment := utils.RandomString(8)
		req, err := json.Marshal(ActionsAcknowledgeProblemRequest{
			Type:    "Host",
			Filter:  fmt.Sprintf(`host.name==%q`, hostname),
			Author:  author,
			Comment: comment,
		})
		ackTime := time.Now()
		require.NoError(t, err, "marshal request")
		response, err := client.PostJson("/v1/actions/acknowledge-problem", bytes.NewBuffer(req))
		require.NoError(t, err, "acknowledge-problem")
		require.Equal(t, 200, response.StatusCode, "acknowledge-problem")

		var ackResponse ActionsAcknowledgeProblemResponse
		err = json.NewDecoder(response.Body).Decode(&ackResponse)
		require.NoError(t, err, "decode acknowledge-problem response")
		require.Equal(t, 1, len(ackResponse.Results), "acknowledge-problem should return 1 result")
		require.Equal(t, http.StatusOK, ackResponse.Results[0].Code, "acknowledge-problem result should have OK status")

		assertEventuallyDrained(t, redisClient, "icinga:history:stream:acknowledgement")

		eventually.Assert(t, func(t require.TestingT) {
			type Row struct {
				Author  string `db:"author"`
				Comment string `db:"comment"`
			}

			var rows []Row
			err = db.Select(&rows, "SELECT a.author, a.comment FROM history h"+
				" JOIN host ON host.id = h.host_id"+
				" JOIN acknowledgement_history a ON a.id = h.acknowledgement_history_id"+
				" WHERE host.name = ? AND ? < h.event_time AND h.event_time < ?",
				hostname, ackTime.Add(-time.Second).UnixMilli(), ackTime.Add(time.Second).UnixMilli())
			require.NoError(t, err, "select acknowledgement_history")

			require.Equal(t, 1, len(rows), "there should be exactly one acknowledgement history entry")
			assert.Equal(t, author, rows[0].Author, "acknowledgement author should match")
			assert.Equal(t, comment, rows[0].Comment, "acknowledgement comment should match")
		}, 5*time.Second, 200*time.Millisecond)
	})

	t.Run("Comment", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_passive_checks": true,
				"check_command":         "dummy",
			},
		})

		author := utils.RandomString(8)
		comment := utils.RandomString(8)
		req, err := json.Marshal(ActionsAddCommentRequest{
			Type:    "Host",
			Filter:  fmt.Sprintf(`host.name==%q`, hostname),
			Author:  author,
			Comment: comment,
		})
		require.NoError(t, err, "marshal request")
		response, err := client.PostJson("/v1/actions/add-comment", bytes.NewBuffer(req))
		require.NoError(t, err, "add-comment")
		require.Equal(t, 200, response.StatusCode, "add-comment")

		var addResponse ActionsAddCommentResponse
		err = json.NewDecoder(response.Body).Decode(&addResponse)
		require.NoError(t, err, "decode add-comment response")
		require.Equal(t, 1, len(addResponse.Results), "add-comment should return 1 result")
		require.Equal(t, http.StatusOK, addResponse.Results[0].Code, "add-comment result should have OK status")

		assertEventuallyDrained(t, redisClient, "icinga:history:stream:comment")

		eventually.Assert(t, func(t require.TestingT) {
			type Row struct {
				Author  string `db:"author"`
				Comment string `db:"comment"`
			}

			var rows []Row
			err = db.Select(&rows, "SELECT c.author, c.comment FROM comment_history c JOIN host ON host.id = c.host_id WHERE host.name = ?", hostname)
			require.NoError(t, err, "select comment_history")

			require.Equal(t, 1, len(rows), "there should be exactly one comment_history row")
			assert.Equal(t, author, rows[0].Author, "author should match")
			assert.Equal(t, comment, rows[0].Comment, "comment text should match")
		}, 5*time.Second, 200*time.Millisecond)
	})

	t.Run("Downtime", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_flapping":       true,
				"enable_passive_checks": true,
				"check_command":         "dummy",
			},
		})

		downtimeStart := time.Now()
		req, err := json.Marshal(ActionsScheduleDowntimeRequest{
			Type:      "Host",
			Filter:    fmt.Sprintf(`host.name==%q`, hostname),
			StartTime: downtimeStart.Unix(),
			EndTime:   downtimeStart.Add(time.Hour).Unix(),
			Fixed:     true,
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
		require.Equal(t, http.StatusOK, scheduleResponse.Results[0].Code, "schedule-downtime result should have OK status")
		downtimeName := scheduleResponse.Results[0].Name

		// Ensure that downtime events have distinct timestamps in millisecond resolution.
		time.Sleep(10 * time.Millisecond)

		req, err = json.Marshal(ActionsRemoveDowntimeRequest{
			Downtime: downtimeName,
		})
		require.NoError(t, err, "marshal remove-downtime request")
		response, err = client.PostJson("/v1/actions/remove-downtime", bytes.NewBuffer(req))
		require.NoError(t, err, "remove-downtime")
		require.Equal(t, 200, response.StatusCode, "remove-downtime")
		downtimeEnd := time.Now()

		assertEventuallyDrained(t, redisClient, "icinga:history:stream:downtime")

		eventually.Assert(t, func(t require.TestingT) {
			var rows []string
			err = db.Select(&rows, "SELECT h.event_type FROM history h"+
				" JOIN host ON host.id = h.host_id"+
				// Joining downtime_history checks that events are written to it.
				" JOIN downtime_history d ON d.downtime_id = h.downtime_history_id"+
				" WHERE host.name = ? AND ? < h.event_time AND h.event_time < ?"+
				" ORDER BY h.event_time",
				hostname, downtimeStart.Add(-time.Second).UnixMilli(), downtimeEnd.Add(time.Second).UnixMilli())
			require.NoError(t, err, "select downtime_history")

			require.Equal(t, []string{"downtime_start", "downtime_end"}, rows,
				"downtime history should match expected result")
		}, 5*time.Second, 200*time.Millisecond)
	})

	t.Run("Flapping", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_flapping":       true,
				"enable_passive_checks": true,
				"check_command":         "dummy",
			},
		})

		timeBefore := time.Now()
		for i := 0; i < 10; i++ {
			processCheckResult(t, client, hostname, 0)
			processCheckResult(t, client, hostname, 1)
		}
		for i := 0; i < 20; i++ {
			processCheckResult(t, client, hostname, 0)
		}
		timeAfter := time.Now()

		assertEventuallyDrained(t, redisClient, "icinga:history:stream:flapping")

		eventually.Assert(t, func(t require.TestingT) {
			var rows []string
			err = db.Select(&rows, "SELECT h.event_type FROM history h"+
				" JOIN host ON host.id = h.host_id"+
				// Joining flapping_history checks that events are written to it.
				" JOIN flapping_history f ON f.id = h.flapping_history_id"+
				" WHERE host.name = ? AND ? < h.event_time AND h.event_time < ?"+
				" ORDER BY h.event_time",
				hostname, timeBefore.Add(-time.Second).UnixMilli(), timeAfter.Add(time.Second).UnixMilli())
			require.NoError(t, err, "select flapping_history")

			require.Equal(t, []string{"flapping_start", "flapping_end"}, rows,
				"flapping history should match expected result")
		}, 5*time.Second, 200*time.Millisecond)
	})

	t.Run("Notification", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_flapping":       true,
				"enable_passive_checks": true,
				"check_command":         "dummy",
				"max_check_attempts":    1,
			},
		})

		users := make([]string, 5)
		for i := range users {
			users[i] = utils.UniqueName(t, "user")
			client.CreateObject(t, "users", users[i], nil)
		}

		// Sort users so that the SQL query can use ORDER BY and the resulting slices can just be compared for equality.
		sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })

		command := utils.UniqueName(t, "notificationcommand")
		client.CreateObject(t, "notificationcommands", command, map[string]interface{}{
			"attrs": map[string]interface{}{
				"command": []string{"true"},
			},
		})

		notification := utils.UniqueName(t, "notification")
		client.CreateObject(t, "notifications", hostname+"!"+notification, map[string]interface{}{
			"attrs": map[string]interface{}{
				"users":   users,
				"command": command,
			},
		})

		type Notification struct {
			Type string `db:"type"`
			User string `db:"username"`
		}

		var expected []Notification

		timeBefore := time.Now()
		processCheckResult(t, client, hostname, 1)
		for _, u := range users {
			expected = append(expected, Notification{Type: "problem", User: u})
		}
		processCheckResult(t, client, hostname, 0)
		for _, u := range users {
			expected = append(expected, Notification{Type: "recovery", User: u})
		}
		timeAfter := time.Now()

		assertEventuallyDrained(t, redisClient, "icinga:history:stream:notification")

		eventually.Assert(t, func(t require.TestingT) {
			var rows []Notification
			err = db.Select(&rows, "SELECT n.type, COALESCE(u.name, '') AS username FROM history h"+
				" JOIN host ON host.id = h.host_id"+
				" JOIN notification_history n ON n.id = h.notification_history_id"+
				" LEFT JOIN user_notification_history un ON un.notification_history_id = n.id"+
				" LEFT JOIN user u ON u.id = un.user_id"+
				" WHERE host.name = ? AND ? < h.event_time AND h.event_time < ?"+
				" ORDER BY h.event_time, username",
				hostname, timeBefore.Add(-time.Second).UnixMilli(), timeAfter.Add(time.Second).UnixMilli())
			require.NoError(t, err, "select notification_history")

			require.Equal(t, expected, rows, "notification history should match expected result")
		}, 5*time.Second, 200*time.Millisecond)
	})

	t.Run("State", func(t *testing.T) {
		t.Parallel()

		hostname := utils.UniqueName(t, "host")
		client.CreateHost(t, hostname, map[string]interface{}{
			"attrs": map[string]interface{}{
				"enable_active_checks":  false,
				"enable_passive_checks": true,
				"check_command":         "dummy",
				"max_check_attempts":    2,
			},
		})

		type State struct {
			Type string `db:"state_type"`
			Soft int    `db:"soft_state"`
			Hard int    `db:"hard_state"`
		}

		var expected []State

		timeBefore := time.Now()
		processCheckResult(t, client, hostname, 0) // UNKNOWN -> UP (hard)
		expected = append(expected, State{Type: "hard", Soft: 0, Hard: 0})
		processCheckResult(t, client, hostname, 1) // -> DOWN (soft)
		expected = append(expected, State{Type: "soft", Soft: 1, Hard: 0})
		processCheckResult(t, client, hostname, 1) // -> DOWN (hard)
		expected = append(expected, State{Type: "hard", Soft: 1, Hard: 1})
		processCheckResult(t, client, hostname, 1) // -> DOWN
		processCheckResult(t, client, hostname, 0) // -> UP (hard)
		expected = append(expected, State{Type: "hard", Soft: 0, Hard: 0})
		processCheckResult(t, client, hostname, 1) // -> DOWN (soft)
		expected = append(expected, State{Type: "soft", Soft: 1, Hard: 0})
		processCheckResult(t, client, hostname, 0) // -> UP (hard)
		expected = append(expected, State{Type: "hard", Soft: 0, Hard: 0})
		processCheckResult(t, client, hostname, 0) // -> UP
		processCheckResult(t, client, hostname, 1) // -> down (soft)
		expected = append(expected, State{Type: "soft", Soft: 1, Hard: 0})
		processCheckResult(t, client, hostname, 1) // -> DOWN (hard)
		expected = append(expected, State{Type: "hard", Soft: 1, Hard: 1})
		processCheckResult(t, client, hostname, 0) // -> UP (hard)
		expected = append(expected, State{Type: "hard", Soft: 0, Hard: 0})
		timeAfter := time.Now()

		assertEventuallyDrained(t, redisClient, "icinga:history:stream:state")

		eventually.Assert(t, func(t require.TestingT) {

			var rows []State
			err = db.Select(&rows, "SELECT s.state_type, s.soft_state, s.hard_state FROM history h"+
				" JOIN host ON host.id = h.host_id JOIN state_history s ON s.id = h.state_history_id"+
				" WHERE host.name = ? AND ? < h.event_time AND h.event_time < ?"+
				" ORDER BY h.event_time",
				hostname, timeBefore.Add(-time.Second).UnixMilli(), timeAfter.Add(time.Second).UnixMilli())
			require.NoError(t, err, "select state_history")

			require.Equal(t, expected, rows, "state history does not match expected result")
		}, 5*time.Second, 200*time.Millisecond)
	})
}

func assertEventuallyDrained(t *testing.T, redis *redis.Client, stream string) {
	eventually.Assert(t, func(t require.TestingT) {
		result, err := redis.XRange(context.Background(), stream, "-", "+").Result()
		require.NoError(t, err, "reading %s should not fail", stream)
		assert.Empty(t, result, "%s should eventually be drained", stream)
	}, 5*time.Second, 10*time.Millisecond)
}

func processCheckResult(t *testing.T, client *utils.Icinga2Client, hostname string, status int) time.Time {
	// Ensure that check results have distinct timestamps in millisecond resolution.
	time.Sleep(10 * time.Millisecond)

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
	if !assert.Equal(t, 200, response.StatusCode, "process-check-result") {
		body, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err, "reading process-check-result response")
		it.Logger(t).Error("process-check-result", zap.ByteString("api-response", body))
		t.FailNow()
	}

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
	require.Equal(t, status, host.Attrs.State, "state should match check result")

	sec, nsec := math.Modf(host.Attrs.LastCheckResult.ExecutionEnd)
	return time.Unix(int64(sec), int64(nsec*1e9))
}

type ActionsAcknowledgeProblemRequest struct {
	Type    string `json:"type"`
	Filter  string `json:"filter"`
	Author  string `json:"author"`
	Comment string `json:"comment"`
}

type ActionsAcknowledgeProblemResponse struct {
	Results []struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
	} `json:"results"`
}

type ActionsAddCommentRequest struct {
	Type    string  `json:"type"`
	Filter  string  `json:"filter"`
	Author  string  `json:"author"`
	Comment string  `json:"comment"`
	Expiry  float64 `json:"expiry"`
}

type ActionsAddCommentResponse struct {
	Results []struct {
		Code     int    `json:"code"`
		LegacyId int    `json:"legacy_id"`
		Name     string `json:"name"`
		Status   string `json:"status"`
	} `json:"results"`
}

type ActionsProcessCheckResultRequest struct {
	Type         string `json:"type"`
	Filter       string `json:"filter"`
	ExitStatus   int    `json:"exit_status"`
	PluginOutput string `json:"plugin_output"`
}

type ActionsRemoveDowntimeRequest struct {
	Downtime string `json:"downtime"`
}

type ActionsScheduleDowntimeRequest struct {
	Type      string  `json:"type"`
	Filter    string  `json:"filter"`
	StartTime int64   `json:"start_time"`
	EndTime   int64   `json:"end_time"`
	Fixed     bool    `json:"fixed"`
	Duration  float64 `json:"duration"`
	Author    string  `json:"author"`
	Comment   string  `json:"comment"`
}

type ActionsScheduleDowntimeResponse struct {
	Results []struct {
		Code   int    `json:"code"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"results"`
}

type ObjectsHostsResponse struct {
	Results []struct {
		Attrs struct {
			State           int `json:"state"`
			StateType       int `json:"state_type"`
			LastCheckResult struct {
				ExecutionEnd float64 `json:"execution_end"`
				ExitStatus   int     `json:"exit_status"`
				Output       string  `json:"output"`
			} `json:"last_check_result"`
			LastHardState       int     `json:"last_hard_state"`
			LastHardStateChange float64 `json:"last_hard_state_change"`
			LastState           int     `json:"last_state"`
		} `json:"attrs"`
	} `json:"results"`
}
