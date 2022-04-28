package icingadb_test

import (
	"encoding/binary"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const MaxPlaceholders = 1 << 13

func TestCleanupAndRetention(t *testing.T) {
	rdb := getDatabase(t)
	db, err := sqlx.Open(rdb.Driver(), rdb.DSN())
	require.NoError(t, err, "connecting to SQL database shouldn't fail")
	t.Cleanup(func() { _ = db.Close() })

	reten := retention{
		Days: 7,
		Options: map[string]int{
			"acknowledgement": 0, // No cleanup.
			"comment":         1,
			"downtime":        2,
			// notification and state default to 7.
		},
	}

	rowsToDelete := 10000
	rowsToSpare := 1000

	for category, stmt := range retentionStatements {
		err := dropNotNullColumns(db, stmt)
		assert.NoError(t, err)

		retentionDays, ok := reten.Options[category]
		if !ok {
			retentionDays = reten.Days
		}

		start := time.Now().AddDate(0, 0, -retentionDays).Add(-1 * time.Millisecond * time.Duration(rowsToDelete))
		startMilli := start.UnixMilli()

		type row struct {
			Id   []byte
			Time int64
		}
		values := make([]row, 0, MaxPlaceholders)
		for j := 0; j < (rowsToDelete + rowsToSpare); j++ {
			if j == rowsToDelete {
				startMilli += int64(2 * time.Minute)
			}

			id := make([]byte, 20)
			binary.LittleEndian.PutUint64(id, uint64(j))

			values = append(values, row{id[:], startMilli + int64(j)})

			if len(values) == MaxPlaceholders || j == (rowsToDelete+rowsToSpare-1) {
				_, err := db.NamedExec(fmt.Sprintf(`INSERT INTO %s (%s, %s) VALUES (:id, :time)`, stmt.Table, stmt.PK, stmt.Column), values)
				require.NoError(t, err)
				values = values[:0]
			}
		}
	}

	r := it.RedisServerT(t)
	i := it.Icinga2NodeT(t, "master")
	i.EnableIcingaDb(r)
	i.Reload()
	waitForDumpDoneSignal(t, r, 20*time.Second, 100*time.Millisecond)
	config, err := yaml.Marshal(struct {
		Retention retention `yaml:"history-retention"`
	}{reten})
	assert.NoError(t, err)
	it.IcingaDbInstanceT(t, r, rdb, services.WithIcingaDbConfig(string(config)))

	eventually.Assert(t, func(t require.TestingT) {
		for category, stmt := range retentionStatements {
			retentionDays, ok := reten.Options[category]
			if !ok {
				retentionDays = reten.Days
			}

			threshold := time.Now().AddDate(0, 0, -retentionDays)
			thresholdMilli := threshold.UnixMilli()

			var rowsLeft int
			err := db.QueryRow(
				db.Rebind(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s < ?`, stmt.Table, stmt.Column)),
				thresholdMilli,
			).Scan(&rowsLeft)
			assert.NoError(t, err)

			var rowsSpared int
			err = db.QueryRow(
				db.Rebind(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s >= ?`, stmt.Table, stmt.Column)),
				thresholdMilli,
			).Scan(&rowsSpared)
			assert.NoError(t, err)

			if retentionDays == 0 {
				// No cleanup.
				assert.Equal(t, rowsToDelete+rowsToSpare, rowsLeft+rowsSpared, "all rows should still be there")
			} else {
				assert.Equal(t, 0, rowsLeft, "rows left in retention period")
				assert.Equal(t, rowsToSpare, rowsSpared, "rows spared")
			}
		}
	}, time.Minute, time.Second)
}

type cleanupStmt struct {
	Table  string
	PK     string
	Column string
}

type retention struct {
	Days    int            `yaml:"days"`
	Options map[string]int `yaml:"options"`
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
WHERE table_schema = %s AND table_name = ? AND column_name NOT IN (?, ?) AND is_nullable = ?`,
		schema)),
		stmt.Table, stmt.PK, stmt.Column, "NO")
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
