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
	return &Service{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Service)(nil)
)
