package icingadb_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icinga-testing/utils"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/icinga/icinga-testing/utils/pki"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"testing"
	"text/template"
	"time"
)

//go:embed history_test_zones.conf
var historyZonesConfRaw string
var historyZonesConfTemplate = template.Must(template.New("zones.conf").Parse(historyZonesConfRaw))

func TestHistory(t *testing.T) {
	t.Run("SingleNode", func(t *testing.T) {
		testHistory(t, 1)
	})

	t.Run("HA", func(t *testing.T) {
		testHistory(t, 2)
	})
}

func testHistory(t *testing.T, numNodes int) {
	m := it.MysqlDatabaseT(t)
	m.ImportIcingaDbSchema()

	ca, err := pki.NewCA()
	require.NoError(t, err, "generating a CA should succeed")

	type Node struct {
		Name         string
		Icinga2      services.Icinga2
		IcingaClient *utils.Icinga2Client
		Redis        services.RedisServer
		RedisClient  *redis.Client
	}

	nodes := make([]*Node, numNodes)

	for i := range nodes {
		name := fmt.Sprintf("master-%d", i)
		redisServer := it.RedisServerT(t)
		icinga := it.Icinga2NodeT(t, name)

		nodes[i] = &Node{
			Name:         name,
			Icinga2:      icinga,
			IcingaClient: icinga.ApiClient(),
			Redis:        redisServer,
			RedisClient:  redisServer.Open(),
		}
	}

	zonesConf := bytes.NewBuffer(nil)
	err = historyZonesConfTemplate.Execute(zonesConf, nodes)
	require.NoError(t, err, "failed to render zones.conf")

	for _, n := range nodes {
		cert, err := ca.NewCertificate(n.Name)
		require.NoError(t, err, "generating cert for %q should succeed", n.Name)
		n.Icinga2.WriteConfig("etc/icinga2/zones.conf", zonesConf.Bytes())
		n.Icinga2.WriteConfig("etc/icinga2/features-available/api.conf", []byte(`
			object ApiListener "api" {
				accept_config = true
				accept_commands = true
			}
		`))
		n.Icinga2.WriteConfig("var/lib/icinga2/certs/ca.crt", ca.CertificateToPem())
		n.Icinga2.WriteConfig("var/lib/icinga2/certs/"+n.Name+".crt", cert.CertificateToPem())
		n.Icinga2.WriteConfig("var/lib/icinga2/certs/"+n.Name+".key", cert.KeyToPem())
		n.Icinga2.EnableIcingaDb(n.Redis)
		err = n.Icinga2.Reload()
		require.NoError(t, err, "icinga2 should reload without error")
		it.IcingaDbInstanceT(t, n.Redis, m)

		{
			n := n
			t.Cleanup(func() { _ = n.RedisClient.Close() })
		}
	}

	eventually.Require(t, func(t require.TestingT) {
		for i, ni := range nodes {
			for j, nj := range nodes {
				if i != j {
					response, err := ni.IcingaClient.GetJson("/v1/objects/endpoints/" + nj.Name)
					require.NoErrorf(t, err, "fetching endpoint %q from %q should not fail", nj.Name, ni.Name)
					require.Equalf(t, 200, response.StatusCode, "fetching endpoint %q from %q should not fail", nj.Name, ni.Name)

					var endpoints ObjectsEndpointsResponse
					err = json.NewDecoder(response.Body).Decode(&endpoints)
					require.NoErrorf(t, err, "parsing response from %q for endpoint %q should not fail", ni.Name, nj.Name)
					require.NotEmptyf(t, endpoints.Results, "response from %q for endpoint %q should contain a result", ni.Name, nj.Name)

					assert.Truef(t, endpoints.Results[0].Attrs.Connected, "endpoint %q should be connected to %q", nj.Name, ni.Name)
				}
			}
		}
	}, 15*time.Second, 200*time.Millisecond)

	db, err := sqlx.Connect("mysql", m.DSN())
	require.NoError(t, err, "connecting to mysql")
	t.Cleanup(func() { _ = db.Close() })

	client := nodes[0].IcingaClient

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

		for _, n := range nodes {
			assertEventuallyDrained(t, n.RedisClient, "icinga:history:stream:acknowledgement")

		}

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

		for _, n := range nodes {
			assertEventuallyDrained(t, n.RedisClient, "icinga:history:stream:comment")
		}

		eventually.Assert(t, func(t require.TestingT) {
			type Row struct {
				Author  string `db:"author"`
				Comment string `db:"comment"`
			}

			var rows []Row
			err = db.Select(&rows, "SELECT c.author, c.comment"+
				" FROM history h"+
				" JOIN comment_history c ON c.comment_id = h.comment_history_id"+
				" JOIN host ON host.id = c.host_id WHERE host.name = ?", hostname)
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

		for _, n := range nodes {
			assertEventuallyDrained(t, n.RedisClient, "icinga:history:stream:downtime")
		}

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

		for _, n := range nodes {
			assertEventuallyDrained(t, n.RedisClient, "icinga:history:stream:flapping")
		}

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

		for _, n := range nodes {
			assertEventuallyDrained(t, n.RedisClient, "icinga:history:stream:notification")
		}

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

		for _, n := range nodes {
			assertEventuallyDrained(t, n.RedisClient, "icinga:history:stream:state")
		}

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

func assertEventuallyDrained(t testing.TB, redis *redis.Client, stream string) {
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

type ObjectsEndpointsResponse struct {
	Results []struct {
		Name  string `json:"name"`
		Attrs struct {
			Connected bool `json:"connected"`
		} `json:"attrs"`
	} `json:"results"`
}
