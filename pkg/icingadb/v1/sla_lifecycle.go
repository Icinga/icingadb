package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type SlaLifecycle struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	HostId                types.Binary    `json:"host_id"`
	ServiceId             types.Binary    `json:"service_id"`
	CreateTime            types.UnixMilli `json:"create_time"`
	DeleteTime            types.UnixMilli `json:"delete_time"`
}

func NewSlaLifecycle() database.Entity {
	return &SlaLifecycle{}
}

// Assert interface compliance.
var (
	_ database.Entity = (*SlaLifecycle)(nil)
)
