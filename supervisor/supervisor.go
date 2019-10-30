package supervisor

import (
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/jsondecoder"
	"sync"
)

type Supervisor struct {
	ChErr    chan error
	ChDecode chan *jsondecoder.JsonDecodePackages
	Rdbw     *connection.RDBWrapper
	Dbw      *connection.DBWrapper
	EnvId    []byte
	EnvLock  *sync.Mutex
}
