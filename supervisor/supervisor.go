package supervisor

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"git.icinga.com/icingadb/icingadb-ha"
)

type Supervisor struct {
	ChErr chan error
	ChEnv chan *icingadb_ha.Environment
	ChDecode chan *icingadb_json_decoder.JsonDecodePackage
	Rdbw *icingadb_connection.RDBWrapper
	Dbw  *icingadb_connection.DBWrapper
}