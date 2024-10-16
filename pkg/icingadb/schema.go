package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/pkg/errors"
	"time"
)

const (
	expectedMysqlSchemaVersion    = 6
	expectedPostgresSchemaVersion = 4
)

// CheckSchema asserts the database schema of the expected version being present.
//
// Icinga DB uses incremental schema updates. Each schema version is identified by a continuous rising positive integer.
// With the initial schema import, the schema version of that time will be inserted in a row within the icingadb_schema
// table. Each subsequent schema update inserts another row with its version number. To have a consistent database
// schema, each schema update must be applied. NOTE: This might change in the future.
func CheckSchema(ctx context.Context, db *database.DB) error {
	var expectedDbSchemaVersion uint16
	switch db.DriverName() {
	case database.MySQL:
		expectedDbSchemaVersion = expectedMysqlSchemaVersion
	case database.PostgreSQL:
		expectedDbSchemaVersion = expectedPostgresSchemaVersion
	}

	var versions []uint16

	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			query := "SELECT version FROM icingadb_schema ORDER BY version ASC"
			err = db.SelectContext(ctx, &versions, query)
			if err != nil {
				err = database.CantPerformQuery(err, query)
			}
			return
		},
		retry.Retryable,
		backoff.NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute),
		db.GetDefaultRetrySettings())
	if err != nil {
		return errors.Wrap(err, "can't check database schema version")
	}

	if len(versions) == 0 {
		return fmt.Errorf("no database schema version is stored in the database")
	}

	// Check if each schema update between the initial import and the latest version was applied or, in other words,
	// that no schema update was left out. The loop goes over the ascending sorted array of schema versions, verifying
	// that each element's successor is the increment of this version, ensuring no gaps in between.
	for i := 0; i < len(versions)-1; i++ {
		if versions[i] != versions[i+1]-1 {
			return fmt.Errorf(
				"incomplete database schema upgrade: intermediate version v%d is missing,"+
					" please make sure you have applied all database migrations after upgrading Icinga DB",
				versions[i]+1)
		}
	}

	if latestVersion := versions[len(versions)-1]; latestVersion != expectedDbSchemaVersion {
		// Since these error messages are trivial and mostly caused by users, we don't need
		// to print a stack trace here. However, since errors.Errorf() does this automatically,
		// we need to use fmt instead.
		return fmt.Errorf(
			"unexpected database schema version: v%d (expected v%d), please make sure you have applied all database"+
				" migrations after upgrading Icinga DB", latestVersion, expectedDbSchemaVersion,
		)
	}

	return nil
}
