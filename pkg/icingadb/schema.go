package icingadb

import (
	"context"
	stderrors "errors"
	"fmt"
	"maps"
	"os"
	"path"
	"slices"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icingadb/internal"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// ErrSchemaNotExists implies that no Icinga DB schema has been imported.
var ErrSchemaNotExists = stderrors.New("no database schema exists")

// ErrSchemaMismatch implies an unexpected schema version, most likely after Icinga DB was updated but the database
// missed the schema upgrade.
var ErrSchemaMismatch = stderrors.New("unexpected database schema version")

// ErrSchemaImperfect implies some non critical failure condition of the database schema.
var ErrSchemaImperfect = stderrors.New("imperfect database schema")

// CheckSchema verifies the correct database schema is present.
//
// This function returns the following error types, possibly wrapped:
//   - If no schema exists, the error returned is ErrSchemaNotExists.
//   - If the schema version does not match the expected version, the error returned is ErrSchemaMismatch.
//   - If there are non fatal database schema conditions, ErrSchemaImperfect is returned. This error must
//     be reported back to the user, but should not lead in a program termination.
//   - Otherwise, the original error is returned, for example in case of general database problems.
func CheckSchema(ctx context.Context, db *database.DB) error {
	var schemaVersions map[uint16]string
	switch db.DriverName() {
	case database.MySQL:
		schemaVersions = internal.MySqlSchemaVersions
	case database.PostgreSQL:
		schemaVersions = internal.PgSqlSchemaVersions
	default:
		return errors.Errorf("unsupported database driver %q", db.DriverName())
	}

	expectedDbSchemaVersion := slices.Max(slices.Sorted(maps.Keys(schemaVersions)))

	if hasSchemaTable, err := db.HasTable(ctx, "icingadb_schema"); err != nil {
		return errors.Wrap(err, "can't verify existence of database schema table")
	} else if !hasSchemaTable {
		return ErrSchemaNotExists
	}

	var versions []uint16

	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			query := "SELECT version FROM icingadb_schema ORDER BY id ASC"
			if err := db.SelectContext(ctx, &versions, query); err != nil {
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

	// In the following, multiple error conditions are checked.
	//
	// Since their error messages are trivial and mostly caused by users, we don't need
	// to print a stack trace here. However, since errors.Errorf() does this automatically,
	// we need to use fmt.Errorf() instead.

	// Check if any schema was imported.
	if len(versions) == 0 {
		return fmt.Errorf("%w: no database schema version is stored in the database", ErrSchemaMismatch)
	}

	// Check if the latest schema version was imported.
	if latestVersion := slices.Max(versions); latestVersion != expectedDbSchemaVersion {
		return fmt.Errorf("%w: v%d (expected v%d), "+
			"please apply the %s.sql schema upgrade file to your database after upgrading Icinga DB: "+
			"https://icinga.com/docs/icinga-db/latest/doc/04-Upgrading/",
			ErrSchemaMismatch, latestVersion, expectedDbSchemaVersion, schemaVersions[expectedDbSchemaVersion])
	}

	// Check if all schema updates between the oldest schema version and the expected version were applied.
	for version := slices.Min(versions); version < expectedDbSchemaVersion; version++ {
		if !slices.Contains(versions, version) {
			release := "UNKNOWN"
			if releaseVersion, ok := schemaVersions[version]; ok {
				release = releaseVersion
			}

			return fmt.Errorf(
				"%w: incomplete database schema upgrade: intermediate version v%d (%s) is missing, "+
					"please inspect the icingadb_schema database table and ensure that all database "+
					"migrations were applied in order after upgrading Icinga DB",
				ErrSchemaMismatch, version, release)
		}
	}

	// Extend the prior check by checking if the schema updates were applied in a monotonic increasing order.
	// However, this returns an ErrSchemaImperfect error instead of an ErrSchemaMismatch.
	for i := 0; i < len(versions)-1; i++ {
		if versions[i] != versions[i+1]-1 {
			return fmt.Errorf(
				"%w: unexpected schema upgrade order after schema version %d, "+
					"please inspect the icingadb_schema database table and ensure that all database "+
					"migrations were applied in order after upgrading Icinga DB",
				ErrSchemaImperfect, versions[i])
		}
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
