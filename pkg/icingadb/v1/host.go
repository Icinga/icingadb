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
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	HostId                types.Binary `json:"object_id"`
	CustomvarId           types.Binary `json:"customvar_id"`
}

func NewHost() contracts.Entity {
	return &Host{}
}

func NewHostCustomvar() contracts.Entity {
	return &HostCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Host)(nil)
)
