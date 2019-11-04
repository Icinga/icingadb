// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package supervisor

import (
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/jsondecoder"
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
