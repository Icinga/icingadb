package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/types"
	"time"
)

// CleanupResult is the result of executing a cleanup routine and
// stores how many rows were deleted and how many statements were executed for that.
type CleanupResult struct {
	Count  uint64
	Rounds uint64
}

// CleanupStmt defines information needed to compose cleanup statements.
type CleanupStmt struct {
	Table  string
	PK     string
	Column string
}

// Build assembles the cleanup statement for the specified database driver with the given limit.
func (stmt *CleanupStmt) Build(driverName string, limit uint64) string {
	switch driverName {
	case driver.MySQL, "mysql":
		return fmt.Sprintf(`DELETE FROM %[1]s WHERE %[2]s < :time ORDER BY %[2]s LIMIT %[3]d`, stmt.Table, stmt.Column, limit)
	case driver.PostgreSQL, "postgres":
		return fmt.Sprintf(`WITH rows AS (
SELECT %[1]s FROM %[2]s WHERE %[3]s < :time ORDER BY %[3]s LIMIT %[4]d
)
DELETE FROM %[2]s WHERE %[1]s IN (SELECT %[1]s FROM rows)`, stmt.PK, stmt.Table, stmt.Column, limit)
	default:
		panic(fmt.Sprintf("invalid database type %s", driverName))
	}
}

// CleanupOlderThan deletes all rows with the specified statement that are older than the given time.
// Deletes a maximum of as many rows per round as defined in count.
func (db *DB) CleanupOlderThan(ctx context.Context, stmt CleanupStmt, count uint64, olderThan time.Time) (CleanupResult, error) {
	var counter com.Counter
	defer db.log(ctx, stmt.Build(db.DriverName(), 0), &counter).Stop()

	var rounds uint64

	for {
		q := db.Rebind(stmt.Build(db.DriverName(), count))
		rs, err := db.NamedExecContext(ctx, q, cleanupWhere{types.UnixMilli(olderThan)})
		if err != nil {
			return CleanupResult{}, internal.CantPerformQuery(err, q)
		}

		n, err := rs.RowsAffected()
		if err != nil {
			return CleanupResult{}, err
		}

		counter.Add(uint64(n))

		if n > 0 {
			rounds++
		}

		if n < int64(count) {
			break
		}
	}

	return CleanupResult{counter.Total(), rounds}, nil
}

type cleanupWhere struct {
	Time types.UnixMilli
}
