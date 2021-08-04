package v1

import (
	"github.com/icinga/icingadb/pkg/types"
)

type Environment struct {
	EntityWithoutChecksum `json:",inline"`
	Name                  types.String `json:"name"`
}
