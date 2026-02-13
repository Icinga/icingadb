package internal

import (
	"github.com/icinga/icinga-go-library/version"
)

// Version contains version and Git commit information.
//
// The placeholders are replaced on `git archive` using the `export-subst` attribute.
var Version = version.Version("1.5.1", "$Format:%(describe)$", "$Format:%H$")

// MySqlSchemaVersions maps MySQL/MariaDB schema versions to Icinga DB release version.
//
// Each schema version implies an available schema upgrade, named after the Icinga DB
// version and stored under ./schema/mysql/upgrades.
//
// The largest key implies the latest and expected schema version.
var MySqlSchemaVersions = map[uint16]string{
	2: "1.0.0-rc2",
	3: "1.0.0",
	4: "1.1.1",
	5: "1.2.0",
	6: "1.2.1",
	7: "1.4.0",
}

// PgSqlSchemaVersions maps PostgreSQL schema versions to Icinga DB release version.
//
// Same as MySqlSchemaVersions, but for PostgreSQL instead.
var PgSqlSchemaVersions = map[uint16]string{
	2: "1.1.1",
	3: "1.2.0",
	4: "1.2.1",
	5: "1.4.0",
}
