package overdue

import (
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
)

type ServiceState struct {
	v1.EntityWithoutChecksum
	IsOverdue types.Bool `json:"is_overdue"`
}

func NewServiceState(id string, overdue bool) (contracts.Entity, error) {
	hs := &ServiceState{IsOverdue: types.Bool{
		Bool:  overdue,
		Valid: true,
	}}

	return hs, hs.Id.UnmarshalText([]byte(id))
}

// Assert interface compliance.
var (
	_ contracts.Entity = (*ServiceState)(nil)
)
