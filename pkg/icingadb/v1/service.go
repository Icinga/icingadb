package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/contracts"
)

type Service struct {
	Checkable `json:",inline"`
	HostId    types.Binary `json:"host_id"`
}

type ServiceCustomvar struct {
	CustomvarMeta `json:",inline"`
	ServiceId     types.Binary `json:"service_id"`
}

type ServiceState struct {
	State     `json:",inline"`
	ServiceId types.Binary `json:"service_id"`
	HostId    types.Binary `json:"host_id"`
}

type Servicegroup struct {
	GroupMeta `json:",inline"`
}

type ServicegroupCustomvar struct {
	CustomvarMeta  `json:",inline"`
	ServicegroupId types.Binary `json:"servicegroup_id"`
}

type ServicegroupMember struct {
	MemberMeta     `json:",inline"`
	ServiceId      types.Binary `json:"service_id"`
	ServicegroupId types.Binary `json:"servicegroup_id"`
}

func NewService() database.Entity {
	return &Service{}
}

func NewServiceCustomvar() database.Entity {
	return &ServiceCustomvar{}
}

func NewServiceState() database.Entity {
	return &ServiceState{}
}

func NewServicegroup() database.Entity {
	return &Servicegroup{}
}

func NewServicegroupCustomvar() database.Entity {
	return &ServicegroupCustomvar{}
}

func NewServicegroupMember() database.Entity {
	return &ServicegroupMember{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Service)(nil)
	_ contracts.Initer = (*Servicegroup)(nil)
)
