package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/types"
)

type Endpoint struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameCiMeta         `json:",inline"`
	ZoneId             types.Binary `json:"zone_id"`
}

type Zone struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameCiMeta         `json:",inline"`
	IsGlobal           types.Bool   `json:"is_global"`
	ParentId           types.Binary `json:"parent_id"`
	Depth              uint8        `json:"depth"`
}

func NewEndpoint() database.Entity {
	return &Endpoint{}
}

func NewZone() database.Entity {
	return &Zone{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Endpoint)(nil)
	_ contracts.Initer = (*Zone)(nil)
)
