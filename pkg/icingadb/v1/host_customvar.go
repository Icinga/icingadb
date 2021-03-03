package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type HostCustomvar struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	HostId                types.Binary `json:"object_id"`
	CustomvarId           types.Binary `json:"customvar_id"`
}

func NewHostCustomvar() contracts.Entity {
	cv := &HostCustomvar{}

	return cv
}
