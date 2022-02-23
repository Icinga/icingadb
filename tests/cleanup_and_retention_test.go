package icingadb_test

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

const MaxPlaceholders = 1 << 13

func TestCleanupAndRetention(t *testing.T) {
	database := func(t *testing.T) *icingadb.DB {
		rdb := getEmptyDatabase(t)
		db, err := sqlx.Connect(rdb.Driver(), rdb.DSN())
		require.NoError(t, err, "connecting to database")
		t.Cleanup(func() { _ = db.Close() })

		return icingadb.NewDb(db, logging.NewLogger(it.Logger(t).Sugar(), time.Second), &icingadb.Options{})
	}
	start := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	startMilli := utils.UnixMilli(start)

	scenarios := []struct {
		limit        int64
		rowsToDelete int64
		rowsToSpare  int64
	}{
		{10, 9, 100},
		{10, 10, 100},
		{10, 100, 100},
		{10, 1000, 100},
		{100, 100, 100},
		{1000, 100000, 100},
	}

	for i := range scenarios {
		scenario := scenarios[i]

		t.Run(fmt.Sprintf("%d rows with limit %d", scenario.rowsToDelete, scenario.limit), func(t *testing.T) {
			t.Parallel()

			table := fmt.Sprintf("retention_test_limit_%d_delete_%d", scenario.limit, scenario.rowsToDelete)

			db := database(t)

			_, err := db.Exec(fmt.Sprintf(`CREATE TEMPORARY TABLE %s (id int NOT NULL, event_time bigint NOT NULL, PRIMARY KEY (id))`, table))
			require.NoError(t, err)

			_, err = db.Exec(fmt.Sprintf(`CREATE INDEX idx_%[1]s ON %[1]s (event_time)`, table))
			require.NoError(t, err)

			// One row per millisecond.
			total := scenario.rowsToDelete + scenario.rowsToSpare
			type row struct {
				Id   int64
				Time int64
			}
			values := make([]row, 0, MaxPlaceholders)
			for j := int64(0); j < total; j++ {
				values = append(values, row{j, startMilli + j})

				if len(values) == MaxPlaceholders || j == total-1 {
					_, err := db.NamedExec(fmt.Sprintf(`INSERT INTO %s (id, event_time) VALUES (:id, :time)`, table), values)
					require.NoError(t, err)
					values = values[:0]
				}
			}

			cleanup, err := db.CleanupOlderThan(
				context.Background(),
				icingadb.CleanupStmt{Table: table, Column: "event_time", PK: "id"},
				uint64(scenario.limit),
				start.Add(time.Millisecond*time.Duration(scenario.rowsToDelete)),
			)
			assert.NoError(t, err)

			var count uint64
			err = db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count)
			assert.NoError(t, err)
			assert.Equal(t, int(scenario.rowsToSpare), int(count), "rows left in table")
			assert.Equal(t, int(scenario.rowsToDelete), int(cleanup.Count), "deleted rows")
			assert.Equal(t, int((scenario.rowsToDelete+scenario.limit-1)/scenario.limit), int(cleanup.Rounds), "number of rounds")
		})
	}

	t.Run("History", func(t *testing.T) {
		db := database(t)
		retentionDays := 1
		scenario := scenarios[len(scenarios)-1]
		wg := sync.WaitGroup{}

		for _, stmt := range history.RetentionStatements {
			_, err := db.Exec(fmt.Sprintf(
				`CREATE TABLE %[1]s (%[2]s int NOT NULL, %[3]s bigint NOT NULL, PRIMARY KEY (%[2]s))`,
				stmt.Table, stmt.PK, stmt.Column))
			require.NoError(t, err)

			_, err = db.Exec(fmt.Sprintf(`CREATE INDEX idx_%[1]s ON %[1]s (%[2]s)`, stmt.Table, stmt.Column))
			require.NoError(t, err)

			total := scenario.rowsToDelete + scenario.rowsToSpare
			type row struct {
				Id   int64
				Time int64
			}
			values := make([]row, 0, MaxPlaceholders)
			start := time.Now().AddDate(0, 0, -retentionDays).Add(-1 * time.Millisecond * time.Duration(scenario.rowsToDelete))
			startMilli := utils.UnixMilli(start)
			for j := int64(0); j < total; j++ {
				if j == scenario.rowsToDelete {
					startMilli += int64(2 * time.Minute)
				}

				values = append(values, row{j, startMilli + j})

				if len(values) == MaxPlaceholders || j == total-1 {
					_, err := db.NamedExec(fmt.Sprintf(`INSERT INTO %s (%s, %s) VALUES (:id, :time)`, stmt.Table, stmt.PK, stmt.Column), values)
					require.NoError(t, err)
					values = values[:0]
				}
			}

			wg.Add(1)
		}

		ctx, cancelCtx := context.WithCancel(context.Background())
		type result struct {
			table   string
			cleanup icingadb.CleanupResult
		}
		results := make(chan result)
		go func() {
			defer close(results)

			ret := history.NewRetention(db, uint64(retentionDays), time.Hour, uint64(scenario.limit), history.RetentionOptions{}, logging.NewLogger(it.Logger(t).Sugar(), time.Second))
			err := ret.StartWithCallback(ctx, func(t string, rs icingadb.CleanupResult) {
				select {
				case results <- result{t, rs}:
				case <-ctx.Done():
				}
			})
			if !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		}()

		cleanups := make(map[string]icingadb.CleanupResult, len(history.RetentionStatements))
		timeout := time.After(time.Minute)
		for done := false; !done; {
			select {
			case rs := <-results:
				if _, ok := cleanups[rs.table]; ok {
					t.Error("cleanup result for table", rs.table, "already exists")
					done = true
					break
				}

				cleanups[rs.table] = rs.cleanup

				if len(cleanups) == len(history.RetentionStatements) {
					done = true
					break
				}
			case <-timeout:
				t.Error("timeout exceeded")
				done = true
				break
			}
		}

		cancelCtx()

		for table, cleanup := range cleanups {
			var count uint64
			err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count)
			assert.NoError(t, err)
			assert.Equal(t, int(scenario.rowsToSpare), int(count), "rows left in table")
			assert.Equal(t, int(scenario.rowsToDelete), int(cleanup.Count), "deleted rows")
			assert.Equal(t, int((scenario.rowsToDelete+scenario.limit-1)/scenario.limit), int(cleanup.Rounds), "number of rounds")
		}
	})
}
