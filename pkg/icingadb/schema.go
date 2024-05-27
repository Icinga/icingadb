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
	expectedMysqlSchemaVersion    = 5
	expectedPostgresSchemaVersion = 3
)

// CheckSchema asserts the database schema of the expected version being present.
func CheckSchema(ctx context.Context, db *database.DB) error {
	var expectedDbSchemaVersion uint16
	switch db.DriverName() {
	case database.MySQL:
		expectedDbSchemaVersion = expectedMysqlSchemaVersion
	case database.PostgreSQL:
		expectedDbSchemaVersion = expectedPostgresSchemaVersion
	}

	var version uint16

	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			query := "SELECT version FROM icingadb_schema ORDER BY id DESC LIMIT 1"
			err = db.QueryRowxContext(ctx, query).Scan(&version)
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

	if version != expectedDbSchemaVersion {
		// Since these error messages are trivial and mostly caused by users, we don't need
		// to print a stack trace here. However, since errors.Errorf() does this automatically,
		// we need to use fmt instead.
		return fmt.Errorf(
			"unexpected database schema version: v%d (expected v%d), please make sure you have applied all database"+
				" migrations after upgrading Icinga DB", version, expectedDbSchemaVersion,
		)
	}

	return nil
}
