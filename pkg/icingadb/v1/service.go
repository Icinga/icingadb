package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Service struct {
	Checkable `json:",inline"`
	HostId    types.Binary `json:"host_id"`
}

func NewService() contracts.Entity {
	s := &Service{}
	// TODO(el): Interfacify!
	s.NameCi = &s.Name

	return s
}
