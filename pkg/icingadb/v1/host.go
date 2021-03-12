package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Host struct {
	Checkable `json:",inline"`
	Address   string `json:"address"`
	Address6  string `json:"address6"`
}

type HostCustomvar struct {
	CustomvarMeta `json:",inline"`
	HostId        types.Binary `json:"object_id"`
}

type Hostgroup struct {
	GroupMeta `json:",inline"`
}

type HostgroupCustomvar struct {
	CustomvarMeta `json:",inline"`
	HostgroupId   types.Binary `json:"object_id"`
}

type HostgroupMember struct {
	MemberMeta  `json:",inline"`
	HostId      types.Binary `json:"object_id"`
	HostgroupId types.Binary `json:"group_id"`
}

func NewHost() contracts.Entity {
	return &Host{}
}

func NewHostCustomvar() contracts.Entity {
	return &HostCustomvar{}
}

func NewHostgroup() contracts.Entity {
	return &Hostgroup{}
}

func NewHostgroupCustomvar() contracts.Entity {
	return &HostgroupCustomvar{}
}

func NewHostgroupMember() contracts.Entity {
	return &HostgroupMember{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Host)(nil)
	_ contracts.Initer = (*Hostgroup)(nil)
)
