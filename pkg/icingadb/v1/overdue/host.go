package overdue

import (
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
)

type HostState struct {
	v1.EntityWithoutChecksum
	IsOverdue types.Bool `json:"is_overdue"`
}

func NewHostState(id string, overdue bool) (contracts.Entity, error) {
	hs := &HostState{IsOverdue: types.Bool{
		Bool:  overdue,
		Valid: true,
	}}

	return hs, hs.Id.UnmarshalText([]byte(id))
}

// Assert interface compliance.
var (
	_ contracts.Entity = (*HostState)(nil)
)
