package icingadb

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/pkg/errors"
	"os"
	"path"
	"time"
)

const (
	expectedMysqlSchemaVersion    = 6
	expectedPostgresSchemaVersion = 4
)

var (
	// ErrSchemaNotExists implies that no Icinga DB schema has been imported.
	ErrSchemaNotExists = fmt.Errorf("no database schema exists")

	// ErrSchemaMismatch implies an unexpected schema version, most likely after Icinga DB was updated but the database
	// missed the schema upgrade.
	ErrSchemaMismatch = fmt.Errorf("unexpected database schema version")
)

// CheckSchema verifies the correct database schema is present.
//
// This function returns the following error types, possibly wrapped:
//   - If no schema exists, the error returned is ErrSchemaNotExists.
//   - If the schema version does not match the expected version, the error returned is ErrSchemaMismatch.
//   - Otherwise, the original error is returned, e.g., for general database problems.
func CheckSchema(ctx context.Context, db *database.DB, databaseName string) error {
	var (
		tableSchema             string
		expectedDbSchemaVersion uint16
	)
	switch db.DriverName() {
	case database.MySQL:
		tableSchema = databaseName
		expectedDbSchemaVersion = expectedMysqlSchemaVersion

	case database.PostgreSQL:
		tableSchema = "public"
		expectedDbSchemaVersion = expectedPostgresSchemaVersion

	default:
		return fmt.Errorf("unsupported database driver %q", db.DriverName())
	}

	var tableSchemaCount int
	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			// The following query should return either 0 or 1 depending on the existence of the icingadb_schema table.
			// Unfortunately, a "SELECT 1" query does not work, because at least the PostgreSQL driver does not raise an
			// error for empty results.
			query := db.Rebind("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME='icingadb_schema'")
			if err := db.QueryRowxContext(ctx, query, tableSchema).Scan(&tableSchemaCount); err != nil {
				return database.CantPerformQuery(err, query)
			}
			return nil
		},
		retry.Retryable,
		backoff.NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute),
		db.GetDefaultRetrySettings())
	if err != nil {
		return errors.Wrap(err, "can't verify existence of database schema table")
	}
	if tableSchemaCount == 0 {
		return ErrSchemaNotExists
	}
	if tableSchemaCount != 1 {
		return fmt.Errorf("no or one 'icingadb_schema' tables are expected, found %d", tableSchemaCount)
	}

	var version uint16
	err = retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			query := "SELECT version FROM icingadb_schema ORDER BY id DESC LIMIT 1"
			if err := db.QueryRowxContext(ctx, query).Scan(&version); err != nil {
				return database.CantPerformQuery(err, query)
			}
			return nil
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
		return fmt.Errorf("%w: v%d (expected v%d), please make sure you have applied all database"+
			" migrations after upgrading Icinga DB", ErrSchemaMismatch, version, expectedDbSchemaVersion,
		)
	}

	return nil
}

// ImportSchema performs an initial schema import in the db.
//
// This function assumes that no schema exists. So it should only be called after a prior CheckSchema call.
func ImportSchema(
	ctx context.Context,
	db *database.DB,
	databaseSchemaDir string,
) error {
	var schemaFileDirPart string
	switch db.DriverName() {
	case database.MySQL:
		schemaFileDirPart = "mysql"
	case database.PostgreSQL:
		schemaFileDirPart = "pgsql"
	default:
		return fmt.Errorf("unsupported database driver %q", db.DriverName())
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "can't start database transaction")
	}
	defer func() { _ = tx.Rollback() }()

	schemaFile := path.Join(databaseSchemaDir, schemaFileDirPart, "schema.sql")
	schema, err := os.ReadFile(schemaFile)
	if err != nil {
		return errors.Wrapf(err, "can't open schema file %q", schema)
	}

	switch db.DriverName() {
	case database.MySQL:
		for _, query := range database.MysqlSplitStatements(string(schema)) {
			err := retry.WithBackoff(
				ctx,
				func(ctx context.Context) error {
					if _, err := tx.ExecContext(ctx, query); err != nil {
						return database.CantPerformQuery(err, query)
					}
					return nil
				},
				retry.Retryable,
				backoff.NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute),
				db.GetDefaultRetrySettings())
			if err != nil {
				return errors.Wrap(err, "can't perform schema import")
			}
		}

	case database.PostgreSQL:
		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error {
				if _, err := tx.ExecContext(ctx, string(schema)); err != nil {
					return err
				}
				return nil
			},
			retry.Retryable,
			backoff.NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute),
			db.GetDefaultRetrySettings())
		if err != nil {
			return errors.Wrap(err, "can't perform schema import")
		}

	default:
		return fmt.Errorf("unsupported database driver %q", db.DriverName())
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "can't commit transaction")
	}
	return nil
}
