package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
)

type Customvar struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NameMeta              `json:",inline"`
	Value                 string `json:"value"`
}

func NewCustomvar() contracts.Entity {
	return &Customvar{}
}
