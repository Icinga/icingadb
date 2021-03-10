package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Service struct {
	Checkable `json:",inline"`
	HostId    types.Binary `json:"host_id"`
}

type ServiceCustomvar struct {
	CustomvarMeta `json:",inline"`
	ServiceId     types.Binary `json:"object_id"`
}

func NewService() contracts.Entity {
	return &Service{}
}

func NewServiceCustomvar() contracts.Entity {
	return &ServiceCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Service)(nil)
)
