package testbackends

import (
	"fmt"
	"github.com/Icinga/icingadb/config"
	"os"
)

func GetDbInfo() (driver string, info *config.DbInfo, err error) {
	switch typ := os.Getenv("ICINGADB_TEST_DB_TYPE"); typ {
	case "mysql":
		driver = "mysql"
	case "pgsql":
		driver = "postgres"
	default:
		err = fmt.Errorf("Bad database type: %#v", typ)
		return
	}

	info = &config.DbInfo{
		Host:         os.Getenv("ICINGADB_TEST_DB_HOST"),
		Port:         os.Getenv("ICINGADB_TEST_DB_PORT"),
		Database:     os.Getenv("ICINGADB_TEST_DB_DATABASE"),
		User:         os.Getenv("ICINGADB_TEST_DB_USER"),
		Password:     os.Getenv("ICINGADB_TEST_DB_PASSWORD"),
		MaxOpenConns: 50,
	}
	return
}
