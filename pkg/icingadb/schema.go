package icingadb

import (
	"context"
	stderrors "errors"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"os"
	"path"
)

const (
	expectedMysqlSchemaVersion    = 7
	expectedPostgresSchemaVersion = 5
)

// ErrSchemaNotExists implies that no Icinga DB schema has been imported.
var ErrSchemaNotExists = stderrors.New("no database schema exists")

// ErrSchemaMismatch implies an unexpected schema version, most likely after Icinga DB was updated but the database
// missed the schema upgrade.
var ErrSchemaMismatch = stderrors.New("unexpected database schema version")

// CheckSchema verifies the correct database schema is present.
//
// This function returns the following error types, possibly wrapped:
//   - If no schema exists, the error returned is ErrSchemaNotExists.
//   - If the schema version does not match the expected version, the error returned is ErrSchemaMismatch.
//   - Otherwise, the original error is returned, for example in case of general database problems.
func CheckSchema(ctx context.Context, db *database.DB) error {
	var expectedDbSchemaVersion uint16
	switch db.DriverName() {
	case database.MySQL:
		expectedDbSchemaVersion = expectedMysqlSchemaVersion
	case database.PostgreSQL:
		expectedDbSchemaVersion = expectedPostgresSchemaVersion
	default:
		return errors.Errorf("unsupported database driver %q", db.DriverName())
	}

	if hasSchemaTable, err := db.HasTable(ctx, "icingadb_schema"); err != nil {
		return errors.Wrap(err, "can't verify existence of database schema table")
	} else if !hasSchemaTable {
		return ErrSchemaNotExists
	}

	var version uint16
	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			query := "SELECT version FROM icingadb_schema ORDER BY id DESC LIMIT 1"
			if err := db.QueryRowxContext(ctx, query).Scan(&version); err != nil {
				return database.CantPerformQuery(err, query)
			}
			return nil
		},
		retry.Retryable,
		backoff.DefaultBackoff,
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
//
// Note: Running a schema file may have side effects, such as altering SQL system variables. Unless you are certain that
// the schema update will not interfere with future queries, consider using a dedicated database connection.
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
		return errors.Errorf("unsupported database driver %q", db.DriverName())
	}

	schemaFile := path.Join(databaseSchemaDir, schemaFileDirPart, "schema.sql")
	schema, err := os.ReadFile(schemaFile) // #nosec G304 -- path is constructed from "trusted" command line user input
	if err != nil {
		return errors.Wrapf(err, "can't open schema file %q", schemaFile)
	}

	queries := []string{string(schema)}
	if db.DriverName() == database.MySQL {
		// MySQL/MariaDB requires the schema to be imported on a statement by statement basis.
		queries = database.MysqlSplitStatements(string(schema))
	}

	return errors.Wrapf(db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		for _, query := range queries {
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return errors.Wrap(database.CantPerformQuery(err, query), "can't perform schema import")
			}
		}
		return nil
	}), "can't import database schema from %q", schemaFile)
}
