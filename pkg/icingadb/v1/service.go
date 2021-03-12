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

type Servicegroup struct {
	GroupMeta `json:",inline"`
}

type ServicegroupCustomvar struct {
	CustomvarMeta  `json:",inline"`
	ServicegroupId types.Binary `json:"object_id"`
}

type ServicegroupMember struct {
	MemberMeta     `json:",inline"`
	ServiceId      types.Binary `json:"object_id"`
	ServicegroupId types.Binary `json:"group_id"`
}

func NewService() contracts.Entity {
	return &Service{}
}

func NewServiceCustomvar() contracts.Entity {
	return &ServiceCustomvar{}
}

func NewServicegroup() contracts.Entity {
	return &Servicegroup{}
}

func NewServicegroupCustomvar() contracts.Entity {
	return &ServicegroupCustomvar{}
}

func NewServicegroupMember() contracts.Entity {
	return &ServicegroupMember{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Service)(nil)
	_ contracts.Initer = (*Servicegroup)(nil)
)
