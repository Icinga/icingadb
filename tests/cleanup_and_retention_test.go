package icingadb_test

import (
	"encoding/binary"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/icinga/icingadb/tests/internal/utils"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestCleanupAndRetention(t *testing.T) {
	r := it.RedisServerT(t)
	i := it.Icinga2NodeT(t, "master")
	i.EnableIcingaDb(r)
	i.Reload()

	rdb := getDatabase(t)
	db, err := sqlx.Open(rdb.Driver(), rdb.DSN())
	require.NoError(t, err, "connecting to SQL database shouldn't fail")
	t.Cleanup(func() { _ = db.Close() })

	reten := retention{
		HistoryDays: 7,
		SlaDays:     30,
		Options: map[string]int{
			"acknowledgement": 0, // No cleanup.
			"comment":         1,
			"downtime":        2,
			// notification and state default to 7.
		},
	}

	daysForCategory := func(category string) int {
		if strings.HasPrefix(category, "sla_") {
			return reten.SlaDays
		} else if d, ok := reten.Options[category]; ok {
			return d
		} else {
			return reten.HistoryDays
		}
	}

	envId := utils.GetEnvironmentIdFromRedis(t, r)
	otherEnvId := append([]byte(nil), envId...)
	otherEnvId[0]++

	rowsToDelete := 10000
	rowsToSpare := 1000
	rowsInOtherEnv := 1000

	for category, stmt := range retentionStatements {
		err := dropNotNullColumns(db, stmt)
		assert.NoError(t, err)

		retentionDays := daysForCategory(category)
		start := time.Now().AddDate(0, 0, -retentionDays)
		startMilli := start.UnixMilli()

		type row struct {
			Env  []byte
			Id   []byte
			Time int64
		}

		nextId := 1
		getId := func() []byte {
			id := make([]byte, 20)
			binary.LittleEndian.PutUint64(id, uint64(nextId))
			nextId++
			return id
		}
		values := make([]row, 0, rowsToDelete+rowsToSpare+rowsInOtherEnv)

		for j := 0; j < rowsToDelete; j++ {
			values = append(values, row{envId, getId(), startMilli - int64(j)})
		}
		for j := 0; j < rowsToSpare; j++ {
			values = append(values, row{envId, getId(), startMilli + (2 * time.Minute).Milliseconds() + int64(j)})
		}
		for j := 0; j < rowsInOtherEnv; j++ {
			values = append(values, row{otherEnvId, getId(), startMilli - int64(j)})
		}

		_, err = db.NamedExec(fmt.Sprintf(`INSERT INTO %s (environment_id, %s, %s) VALUES (:env, :id, :time)`,
			stmt.Table, stmt.PK, stmt.Column), values)
		require.NoError(t, err)
	}

	waitForDumpDoneSignal(t, r, 20*time.Second, 100*time.Millisecond)
	config, err := yaml.Marshal(struct {
		Retention retention `yaml:"retention"`
	}{reten})
	assert.NoError(t, err)
	it.IcingaDbInstanceT(t, r, rdb, services.WithIcingaDbConfig(string(config)))

	eventually.Assert(t, func(t require.TestingT) {
		for category, stmt := range retentionStatements {
			retentionDays := daysForCategory(category)
			threshold := time.Now().AddDate(0, 0, -retentionDays)
			thresholdMilli := threshold.UnixMilli()

			var rowsLeft int
			err := db.QueryRow(
				db.Rebind(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE environment_id = ? AND %s < ?`, stmt.Table, stmt.Column)),
				envId,
				thresholdMilli,
			).Scan(&rowsLeft)
			assert.NoError(t, err)

			var rowsSpared int
			err = db.QueryRow(
				db.Rebind(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE environment_id = ? AND %s >= ?`, stmt.Table, stmt.Column)),
				envId,
				thresholdMilli,
			).Scan(&rowsSpared)
			assert.NoError(t, err)

			if retentionDays == 0 {
				// No cleanup.
				assert.Equal(t, rowsToDelete+rowsToSpare, rowsLeft+rowsSpared, "all rows should still be there for %s", category)
			} else {
				assert.Equal(t, 0, rowsLeft, "rows left in retention period for %s", category)
				assert.Equal(t, rowsToSpare, rowsSpared, "rows spared for %s", category)
			}

			var rowsSparedOtherEnv int
			err = db.QueryRow(
				db.Rebind(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE environment_id <> ?`, stmt.Table)),
				envId,
			).Scan(&rowsSparedOtherEnv)
			assert.NoError(t, err)

			assert.Equal(t, rowsInOtherEnv, rowsSparedOtherEnv, "should not delete rows in other environment for %s", category)
		}
	}, time.Minute, time.Second)
}

type cleanupStmt struct {
	Table  string
	PK     string
	Column string
}

type retention struct {
	HistoryDays int            `yaml:"history-days"`
	SlaDays     int            `yaml:"sla-days"`
	Options     map[string]int `yaml:"options"`
}

var retentionStatements = map[string]cleanupStmt{
	"acknowledgement": {
		Table:  "acknowledgement_history",
		PK:     "id",
		Column: "clear_time",
	},
	"comment": {
		Table:  "comment_history",
		PK:     "comment_id",
		Column: "remove_time",
	},
	"downtime": {
		Table:  "downtime_history",
		PK:     "downtime_id",
		Column: "end_time",
	},
	"flapping": {
		Table:  "flapping_history",
		PK:     "id",
		Column: "end_time",
	},
	"notification": {
		Table:  "notification_history",
		PK:     "id",
		Column: "send_time",
	},
	"state": {
		Table:  "state_history",
		PK:     "id",
		Column: "event_time",
	},
	"sla_downtime": {
		Table:  "sla_history_downtime",
		PK:     "downtime_id",
		Column: "downtime_end",
	},
	"sla_state": {
		Table:  "sla_history_state",
		PK:     "id",
		Column: "event_time",
	},
}

// dropNotNullColumns drops all columns with a NOT NULL constraint that are not
// relevant to testing to simplify the insertion of test fixtures.
func dropNotNullColumns(db *sqlx.DB, stmt cleanupStmt) error {
	var schema string
	switch db.DriverName() {
	case "mysql":
		schema = `SCHEMA()`
	case "postgres":
		schema = `CURRENT_SCHEMA()`
	}

	var cols []string
	err := db.Select(&cols, db.Rebind(fmt.Sprintf(`
SELECT column_name
FROM information_schema.columns
WHERE table_schema = %s AND table_name = ? AND column_name NOT IN (?, ?, ?) AND is_nullable = ?`,
		schema)),
		stmt.Table, "environment_id", stmt.PK, stmt.Column, "NO")
	if err != nil {
		return err
	}
	for i := range cols {
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s DROP COLUMN %s`, stmt.Table, cols[i])); err != nil {
			return err
		}
	}

	return nil
}
