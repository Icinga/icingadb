package utils

import (
	"fmt"
	"github.com/icinga/icinga-testing"
	"github.com/icinga/icinga-testing/services"
	"os"
	"strings"
	"testing"
)

func GetDatabase(it *icingatesting.IT, t testing.TB) services.RelationalDatabase {
	k := "ICINGADB_TESTS_DATABASE_TYPE"
	v := strings.ToLower(os.Getenv(k))

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
