package testbackends

import (
	"fmt"
	"os"
)

var MysqlTestDsn = fmt.Sprintf(
	"%s:%s@tcp(%s:%s)/%s",
	os.Getenv("ICINGADB_TEST_MYSQL_USER"),
	os.Getenv("ICINGADB_TEST_MYSQL_PASSWORD"),
	os.Getenv("ICINGADB_TEST_MYSQL_HOST"),
	os.Getenv("ICINGADB_TEST_MYSQL_PORT"),
	os.Getenv("ICINGADB_TEST_MYSQL_DATABASE"),
)
