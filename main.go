package main

import (
	"flag"
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-ha"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"git.icinga.com/icingadb/icingadb-main/config"
	"git.icinga.com/icingadb/icingadb-main/configobject/host"
	"git.icinga.com/icingadb/icingadb-main/configobject/service"
	"git.icinga.com/icingadb/icingadb-main/configobject/sync"
	"git.icinga.com/icingadb/icingadb-main/prometheus"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	log "github.com/sirupsen/logrus"
)

func main() {
	configPath := flag.String("config", "icingadb.ini", "path to config")
	flag.Parse()

	if err := config.ParseConfig(*configPath); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	redisInfo := config.GetRedisInfo()
	mysqlInfo := config.GetMysqlInfo()

	redisConn, err := icingadb_connection.NewRDBWrapper(redisInfo.Host + ":" + redisInfo.Port)
	if err != nil {
		log.Fatal(err)
	}

	mysqlConn, err := icingadb_connection.NewDBWrapper(mysqlInfo.User + ":" + mysqlInfo.Password + "@tcp(" + mysqlInfo.Host + ":" + mysqlInfo.Port + ")/" + mysqlInfo.Database)
	if err != nil {
		log.Fatal(err)
	}

	super := supervisor.Supervisor{
		ChErr:    make(chan error),
		ChEnv:    make(chan *icingadb_ha.Environment),
		ChDecode: make(chan *icingadb_json_decoder.JsonDecodePackages),
		Rdbw:     redisConn,
		Dbw:      mysqlConn,
	}

	ha := icingadb_ha.HA{}
	go ha.Run(super.Rdbw, super.Dbw, super.ChEnv, super.ChErr)
	go func() {
		super.ChErr <- icingadb_ha.IcingaEventsBroker(redisConn, super.ChEnv)
	}()

	go icingadb_json_decoder.DecodePool(super.ChDecode, super.ChErr, 16)

	chHAHost := ha.RegisterNotificationListener()
	go func() {
		super.ChErr <- sync.Operator(&super, chHAHost, &sync.Context{
			ObjectType: "host",
			Factory:    host.NewHost,
			InsertStmt: host.BulkInsertStmt,
			DeleteStmt: host.BulkDeleteStmt,
			UpdateStmt: host.BulkUpdateStmt,
		})
	}()

	chHAService := ha.RegisterNotificationListener()
	go func() {

		super.ChErr <- sync.Operator(&super, chHAService, &sync.Context{
			ObjectType: "service",
			Factory:    service.NewService,
			InsertStmt: service.BulkInsertStmt,
			DeleteStmt: service.BulkDeleteStmt,
			UpdateStmt: service.BulkUpdateStmt,
		})
	}()

	go prometheus.HandleHttp("0.0.0.0:8080", super.ChErr)

	for {
		select {
		case err := <-super.ChErr:
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
