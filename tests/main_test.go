package icingadb_test

import (
	"fmt"
	"github.com/icinga/icinga-testing"
	"github.com/icinga/icinga-testing/services"
	"os"
	"testing"
)

var it *icingatesting.IT

func TestMain(m *testing.M) {
	it = icingatesting.NewIT()
	defer it.Cleanup()

	m.Run()
}

func getDatabase(t testing.TB) services.RelationalDatabase {
	rdb := getEmptyDatabase(t)

	rdb.ImportIcingaDbSchema()

	return rdb
}

func getEmptyDatabase(t testing.TB) services.RelationalDatabase {
	k := "ICINGADB_TESTS_DATABASE_TYPE"
	v := os.Getenv(k)

	var rdb services.RelationalDatabase

	switch v {
	case "mysql":
		rdb = it.MysqlDatabaseT(t)
	case "pgsql":
		rdb = it.PostgresqlDatabaseT(t)
	default:
		panic(fmt.Sprintf(`unknown database in %s environment variable: %q (must be "mysql" or "pgsql")`, k, v))
	}

	return rdb
}
