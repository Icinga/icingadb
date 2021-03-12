package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Comment struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameMeta           `json:",inline"`
	ObjectType         string          `json:"object_type"`
	HostId             types.Binary    `json:"host_id"`
	ServiceId          types.Binary    `json:"service_id"`
	Author             string          `json:"author"`
	Text               string          `json:"text"`
	EntryType          string          `json:"entry_type"`
	EntryTime          types.UnixMilli `json:"entry_time"`
	IsPersistent       types.Bool      `json:"is_persistent"`
	IsSticky           types.Bool      `json:"is_sticky"`
	ExpireTime         types.UnixMilli `json:"expire_time"`
	ZoneId             types.Binary    `json:"zone_id"`
}

func NewComment() contracts.Entity {
	return &Comment{}
}
