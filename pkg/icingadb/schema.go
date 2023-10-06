package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/pkg/errors"
)

const (
	expectedMysqlSchemaVersion    = 4
	expectedPostgresSchemaVersion = 2
)

// CheckSchema asserts the database schema of the expected version being present.
func CheckSchema(ctx context.Context, db *database.DB) error {
	var expectedDbSchemaVersion uint16
	switch db.DriverName() {
	case driver.MySQL:
		expectedDbSchemaVersion = expectedMysqlSchemaVersion
	case driver.PostgreSQL:
		expectedDbSchemaVersion = expectedPostgresSchemaVersion
	}

	var version uint16

	err := db.QueryRowxContext(ctx, "SELECT version FROM icingadb_schema ORDER BY id DESC LIMIT 1").Scan(&version)
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
