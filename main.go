package main

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-ha"
	"git.icinga.com/icingadb/icingadb-json-decoder"
)

type Supervisor struct {
	chEnv chan *icingadb_connection.Environment
	chDecode chan *icingadb_json_decoder.JsonDecodePackage
	rdbw *icingadb_connection.RDBWrapper
	dbw  *icingadb_connection.DBWrapper
}

func main() {
	chErr := make (chan error)
	redisConn, err := icingadb_connection.NewRDBWrapper("127.0.0.1:6379")
	if err != nil {
		return
	}

	mysqlConn, err := icingadb_connection.NewDBWrapper("module-dev:icinga0815!@tcp(127.0.0.1:3306)/icingadb"	)
	if err != nil {
		return
	}

	super := Supervisor{
		make(chan *icingadb_connection.Environment),
		make(chan *icingadb_json_decoder.JsonDecodePackage),
		redisConn,
		mysqlConn,
	}

	ha := icingadb_ha.HA{}
	ha.Run(super.rdbw, super.dbw, super.chEnv, chErr)
	go func() {
		chErr <- icingadb_ha.IcingaEventsBroker(redisConn, super.chEnv)
	}()

	go icingadb_json_decoder.DecodePool(super.chDecode, chErr, 16)

	go configobject.HostOperator()

	//go create object type supervisors



}