package supervisor

import (
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/jsondecoder"
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
