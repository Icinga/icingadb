package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Timeperiod struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameCiMeta         `json:",inline"`
	DisplayName        string       `json:"display_name"`
	PreferIncludes     types.Bool   `json:"prefer_includes"`
	ZoneId             types.Binary `json:"zone_id"`
}

type TimeperiodRange struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	TimeperiodId          types.Binary `json:"timeperiod_id"`
	RangeKey              string       `json:"range_key"`
	RangeValue            string       `json:"range_value"`
}

type TimeperiodOverrideInclude struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	TimeperiodId          types.Binary `json:"timeperiod_id"`
	OverrideId            types.Binary `json:"include_id"`
}

type TimeperiodOverrideExclude struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	TimeperiodId          types.Binary `json:"timeperiod_id"`
	OverrideId            types.Binary `json:"exclude_id"`
}

type TimeperiodCustomvar struct {
	CustomvarMeta `json:",inline"`
	TimeperiodId  types.Binary `json:"timeperiod_id"`
}

func NewTimeperiod() contracts.Entity {
	return &Timeperiod{}
}

func NewTimeperiodRange() contracts.Entity {
	return &TimeperiodRange{}
}

func NewTimeperiodOverrideInclude() contracts.Entity {
	return &TimeperiodOverrideInclude{}
}

func NewTimeperiodOverrideExclude() contracts.Entity {
	return &TimeperiodOverrideExclude{}
}

func NewTimeperiodCustomvar() contracts.Entity {
	return &TimeperiodCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Timeperiod)(nil)
)
