package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Customvar struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NameMeta              `json:",inline"`
	Value                 string `json:"value"`
}

type CustomvarFlat struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	CustomvarId           types.Binary `json:"customvar_id"`
	Flatname              string       `json:"flatname"`
	FlatnameChecksum      types.Binary `json:"flatname_checksum"`
	Flatvalue             string       `json:"flatvalue"`
}

func NewCustomvar() contracts.Entity {
	return &Customvar{}
}
