package overdue

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
)

type HostState struct {
	v1.EntityWithoutChecksum
	IsOverdue types.Bool `json:"is_overdue"`
}

func NewHostState(id string, overdue bool) (database.Entity, error) {
	hs := &HostState{IsOverdue: types.Bool{
		Bool:  overdue,
		Valid: true,
	}}

	return hs, hs.Id.UnmarshalText([]byte(id))
}

// Assert interface compliance.
var (
	_ database.Entity = (*HostState)(nil)
)
