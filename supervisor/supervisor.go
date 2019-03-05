package supervisor

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-json-decoder"
)

type Supervisor struct {
	ChErr chan error
	ChEnv chan *icingadb_connection.Environment
	ChDecode chan *icingadb_json_decoder.JsonDecodePackage
	Rdbw *icingadb_connection.RDBWrapper
	Dbw  *icingadb_connection.DBWrapper
}