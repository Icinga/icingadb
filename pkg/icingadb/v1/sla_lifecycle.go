package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type SlaLifecycle struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	HostMeta              `json:",inline"`
	ServiceId             types.Binary    `json:"service_id"`
	CreateTime            types.UnixMilli `json:"create_time"`
	DeleteTime            types.UnixMilli `json:"delete_time"`

	// The original checkable entity from which this sla lifecycle were transformed
	SourceEntity contracts.Entity `json:"-" db:"-"`
}

func NewSlaLifecycle() contracts.Entity {
	return &SlaLifecycle{}
}

// Assert interface compliance.
var (
	_ contracts.Entity = (*SlaLifecycle)(nil)
	_ BinaryIDer       = (*SlaLifecycle)(nil)
	_ BinaryHostIDer   = (*SlaLifecycle)(nil)
)
