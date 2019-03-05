package main

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-ha"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
)

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

	super := supervisor.Supervisor{
		ChEnv: make(chan *icingadb_connection.Environment),
		ChDecode: make(chan *icingadb_json_decoder.JsonDecodePackage),
		Rdbw: redisConn,
		Dbw: mysqlConn,
	}

	ha := icingadb_ha.HA{}
	ha.Run(super.Rdbw, super.Dbw, super.ChEnv, chErr)
	go func() {
		chErr <- icingadb_ha.IcingaEventsBroker(redisConn, super.ChEnv)
	}()

	go icingadb_json_decoder.DecodePool(super.ChDecode, chErr, 16)

	go func() {
		chErr <- configobject.HostOperator(&super)
	}()

	//go create object type supervisors



}