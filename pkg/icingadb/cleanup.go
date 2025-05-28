package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"time"
)

// CleanupStmt defines information needed to compose cleanup statements.
type CleanupStmt struct {
	Table  string
	PK     string
	Column string
}

// CleanupOlderThan deletes all rows with the specified statement that are older than the given time.
// Deletes a maximum of as many rows per round as defined in count. Actually deleted rows will be passed to onSuccess.
// Returns the total number of rows deleted.
func (stmt *CleanupStmt) CleanupOlderThan(
	ctx context.Context, db *database.DB, envId types.Binary,
	count uint64, olderThan time.Time, onSuccess ...database.OnSuccess[struct{}],
) (uint64, error) {
	var counter com.Counter

	q := db.Rebind(stmt.build(db.DriverName(), count))

	defer db.Log(ctx, q, &counter).Stop()

	for {
		var rowsDeleted uint64

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				rs, err := db.NamedExecContext(ctx, q, cleanupWhere{
					EnvironmentId: envId,
					Time:          types.UnixMilli(olderThan),
				})
				if err != nil {
					return database.CantPerformQuery(err, q)
				}

				i, err := rs.RowsAffected()
				if err == nil && i >= 0 {
					rowsDeleted = uint64(i)
				}

				return err
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			db.GetDefaultRetrySettings(),
		)
		if err != nil {
			return 0, err
		}

		counter.Add(rowsDeleted)

		for _, onSuccess := range onSuccess {
			if err := onSuccess(ctx, make([]struct{}, rowsDeleted)); err != nil {
				return 0, err
			}
		}

		if rowsDeleted < count {
			break
		}
	}

	return counter.Total(), nil
}

// build assembles the cleanup statement for the specified database driver with the given limit.
func (stmt *CleanupStmt) build(driverName string, limit uint64) string {
	switch driverName {
	case database.MySQL:
		return fmt.Sprintf(`DELETE FROM %[1]s WHERE environment_id = :environment_id AND %[2]s < :time LIMIT %[3]d`,
			stmt.Table, stmt.Column, limit)
	case database.PostgreSQL:
		return fmt.Sprintf(`WITH rows AS (
SELECT %[1]s FROM %[2]s WHERE environment_id = :environment_id AND %[3]s < :time LIMIT %[4]d
)
DELETE FROM %[2]s WHERE %[1]s IN (SELECT %[1]s FROM rows)`, stmt.PK, stmt.Table, stmt.Column, limit)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
	}
}

type cleanupWhere struct {
	EnvironmentId types.Binary
	Time          types.UnixMilli
}
