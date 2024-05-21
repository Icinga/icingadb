package v1

import (
	"github.com/icinga/icingadb/pkg/database"
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
	ScheduledDuration  uint64          `json:"scheduled_duration"`
	IsFlexible         types.Bool      `json:"is_flexible"`
	FlexibleDuration   uint64          `json:"flexible_duration"`
	IsInEffect         types.Bool      `json:"is_in_effect"`
	StartTime          types.UnixMilli `json:"start_time"`
	EndTime            types.UnixMilli `json:"end_time"`
	Duration           uint64          `json:"duration"`
	ScheduledBy        types.String    `json:"scheduled_by"`
	ZoneId             types.Binary    `json:"zone_id"`
}

func NewDowntime() database.Entity {
	return &Downtime{}
}
