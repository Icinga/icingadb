package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type ServiceCustomvar struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	ServiceId             types.Binary `json:"object_id"`
	CustomvarId           types.Binary `json:"customvar_id"`
}

func NewServiceCustomvar() contracts.Entity {
	cv := &ServiceCustomvar{}

	return cv
}
