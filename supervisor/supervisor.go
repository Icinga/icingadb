package supervisor

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"sync"
)

type Supervisor struct {
	ChErr 		chan error
	ChDecode 	chan *icingadb_json_decoder.JsonDecodePackages
	Rdbw		*icingadb_connection.RDBWrapper
	Dbw  		*icingadb_connection.DBWrapper
	EnvId 		[]byte
	EnvLock		*sync.Mutex
}