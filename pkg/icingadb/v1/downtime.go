package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Downtime struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameMeta           `json:",inline"`
	TriggeredById      types.Binary    `json:"triggered_by_id"`
	ParentId           types.Binary    `json:"parent_id"`
	ObjectType         string          `json:"object_type"`
	HostId             types.Binary    `json:"host_id"`
	ServiceId          types.Binary    `json:"service_id"`
	Author             string          `json:"author"`
	Comment            string          `json:"comment"`
	EntryTime          types.UnixMilli `json:"entry_time"`
	ScheduledStartTime types.UnixMilli `json:"scheduled_start_time"`
	ScheduledEndTime   types.UnixMilli `json:"scheduled_end_time"`
	FlexibleDuration   uint64          `json:"flexible_duration"`
	IsFlexible         types.Bool      `json:"is_flexible"`
	IsInEffect         types.Bool      `json:"is_in_effect"`
	StartTime          types.UnixMilli `json:"start_time"`
	EndTime            types.UnixMilli `json:"end_time"`
	ZoneId             types.Binary    `json:"zone_id"`
}

func NewDowntime() contracts.Entity {
	return &Downtime{}
}
