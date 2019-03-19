package main

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-ha"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"git.icinga.com/icingadb/icingadb-main/configobject/host"
	"git.icinga.com/icingadb/icingadb-main/configobject/sync"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	log "github.com/sirupsen/logrus"
)

func main() {
	redisConn, err := icingadb_connection.NewRDBWrapper("127.0.0.1:6379")
	if err != nil {
		log.Fatal(err)
	}

	mysqlConn, err := icingadb_connection.NewDBWrapper("module-dev:icinga0815!@tcp(127.0.0.1:3306)/icingadb"	)
	if err != nil {
		log.Fatal(err)
	}

	super := supervisor.Supervisor{
		ChErr: make (chan error),
		ChEnv: make(chan *icingadb_ha.Environment),
		ChDecode: make(chan *icingadb_json_decoder.JsonDecodePackages),
		Rdbw: redisConn,
		Dbw: mysqlConn,
	}

	ha := icingadb_ha.HA{}
	go ha.Run(super.Rdbw, super.Dbw, super.ChEnv, super.ChErr)
	go func() {
		super.ChErr <- icingadb_ha.IcingaEventsBroker(redisConn, super.ChEnv)
	}()

	go icingadb_json_decoder.DecodePool(super.ChDecode, super.ChErr, 16)

	go func() {
		chHA := ha.RegisterNotificationListener()

		super.ChErr <- sync.Operator(&super, chHA, &sync.Context{
			ObjectType: "host",
			Factory: host.NewHost,
			InsertStmt: host.BulkInsertStmt,
			DeleteStmt: host.BulkDeleteStmt,
			UpdateStmt: host.BulkUpdateStmt,
		})
	}()

	for {
		select {
		case err := <- super.ChErr:
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}