package main

import (
	"git.icinga.com/icingadb-configsync-lib"
	"git.icinga.com/icingadb-connection"
	"git.icinga.com/icingadb-ha-lib"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

var chEnv = make(chan *icingadb_connection.Environment)
var chErr = make(chan error)

func main() {

	dbw := mkMysql()
	rdw := mkRedis()

	log.SetLevel(log.DebugLevel)

	ha := icingadb_ha_lib.HA{}
	go icingadb_ha_lib.IcingaEventsBroker(rdw, chEnv, chErr)
	go ha.Run(rdw, dbw, chEnv, chErr)

	ho := icingadb_configsync_lib.NewHostOperator(dbw, rdw)

	go func() {
		if err := ho.FillChecksumsFromDb(); err != nil {
			chErr <- err
		}
	}()

	var environment= icingadb_connection.Environment{}
	select {
	case env := <-chEnv:
		log.Print(env)
		environment = *env
	case err := <-chErr:
		if err != nil {
			log.Fatal(err)
		}
	}

	if environment.ID == nil {
		log.Fatal("No env!")
	}

	if err := ho.FillAttributesFromRedis(); err != nil {
		log.Fatal(err)
	}

	if err := ho.Sync(); err != nil {
		log.Fatal(err)
	}
}

func mkMysql() *icingadb_connection.DBWrapper {
	dbDsn := "root:foo@tcp(127.0.0.1:3306)/icingadb"
	sep := "?"


	dsnParts := strings.Split(dbDsn, "/")
	if strings.Contains(dsnParts[len(dsnParts)-1], "?") {
		sep = "&"
	}

	dbDsn = dbDsn+ sep +
		"innodb_strict_mode=1&sql_mode='STRICT_ALL_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,NO_ENGINE_SUBSTITUTION,PIPES_AS_CONCAT,ANSI_QUOTES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER'"


	dbw, err := icingadb_connection.NewDBWrapper("mysql", dbDsn)
	if err != nil {
		log.Fatal(err)
	}

	return dbw
}

func mkRedis() *icingadb_connection.RDBWrapper {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:6379",
		DialTimeout:  time.Minute / 2,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})

	rdw := icingadb_connection.NewRDBWrapper(rdb)
	return rdw
}