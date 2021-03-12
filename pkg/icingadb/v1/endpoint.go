package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Endpoint struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameCiMeta         `json:",inline"`
	ZoneId             types.Binary `json:"zone_id"`
}

func NewEndpoint() contracts.Entity {
	return &Endpoint{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Endpoint)(nil)
)
