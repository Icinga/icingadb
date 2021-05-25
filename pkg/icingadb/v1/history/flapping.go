package history

import (
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
)

type FlappingHistoryUpserter struct {
	EndTime               types.UnixMilli `json:"end_time"`
	PercentStateChangeEnd types.Float     `json:"percent_state_change_end"`
	FlappingThresholdLow  float32         `json:"flapping_threshold_low"`
	FlappingThresholdHigh float32         `json:"flapping_threshold_high"`
}

// Upsert implements the contracts.Upserter interface.
func (fhu *FlappingHistoryUpserter) Upsert() interface{} {
	return fhu
}

type FlappingHistory struct {
	v1.EntityWithoutChecksum `json:",inline"`
	HistoryTableMeta         `json:",inline"`
	FlappingHistoryUpserter  `json:",inline"`
	StartTime                types.UnixMilli `json:"start_time"`
	PercentStateChangeStart  types.Float     `json:"percent_state_change_start"`
}

type HistoryFlapping struct {
	HistoryMeta       `json:",inline"`
	FlappingHistoryId types.Binary `json:"id"`

	// Idea: read StartTime and EndTime from Redis and let EventTime decide which of them to write to MySQL.
	// So EventTime doesn't have to be read from Redis (json:"-")
	// and the others don't have to be written to MySQL (db:"-").
	StartTime types.UnixMilli   `json:"start_time" db:"-"`
	EndTime   types.UnixMilli   `json:"end_time" db:"-"`
	EventTime FlappingEventTime `json:"-"`
}

// Init implements the contracts.Initer interface.
func (h *HistoryFlapping) Init() {
	h.EventTime.History = h
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryFlapping) TableName() string {
	return "history"
}

type FlappingEventTime struct {
	History *HistoryFlapping `db:"-"`
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (et FlappingEventTime) Value() (driver.Value, error) {
	if et.History == nil {
		return nil, nil
	}

	switch et.History.EventType {
	case "flapping_start":
		return et.History.StartTime.Value()
	case "flapping_end":
		return et.History.EndTime.Value()
	default:
		return nil, nil
	}
}

// Assert interface compliance.
var (
	_ UpserterEntity       = (*FlappingHistory)(nil)
	_ contracts.Initer     = (*HistoryFlapping)(nil)
	_ contracts.TableNamer = (*HistoryFlapping)(nil)
	_ UpserterEntity       = (*HistoryFlapping)(nil)
	_ driver.Valuer        = FlappingEventTime{}
)
