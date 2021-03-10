package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
)

type Host struct {
	Checkable `json:",inline"`
	Address   string `json:"address"`
	Address6  string `json:"address6"`
}

func NewHost() contracts.Entity {
	h := &Host{}
	// TODO(el): Interfacify!
	h.NameCi = &h.Name

	return h
}
