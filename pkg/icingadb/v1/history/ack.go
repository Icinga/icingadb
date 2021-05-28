package history

import (
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
)

type AckHistoryUpserter struct {
	ClearTime types.UnixMilli `json:"clear_time"`
	ClearedBy types.String    `json:"cleared_by"`
}

// Upsert implements the contracts.Upserter interface.
func (ahu *AckHistoryUpserter) Upsert() interface{} {
	return ahu
}

type AcknowledgementHistory struct {
	v1.EntityWithoutChecksum `json:",inline"`
	HistoryTableMeta         `json:",inline"`
	AckHistoryUpserter       `json:",inline"`
	SetTime                  types.UnixMilli `json:"set_time"`
	Author                   string          `json:"author"`
	Comment                  types.String    `json:"comment"`
	ExpireTime               types.UnixMilli `json:"expire_time"`
	IsPersistent             types.Bool      `json:"is_persistent"`
	IsSticky                 types.Bool      `json:"is_sticky"`
}

type HistoryAck struct {
	HistoryMeta              `json:",inline"`
	AcknowledgementHistoryId types.Binary `json:"id"`

	// Idea: read SetTime and ClearTime from Redis and let EventTime decide which of them to write to MySQL.
	// So EventTime doesn't have to be read from Redis (json:"-")
	// and the others don't have to be written to MySQL (db:"-").
	SetTime   types.UnixMilli `json:"set_time" db:"-"`
	ClearTime types.UnixMilli `json:"clear_time" db:"-"`
	EventTime AckEventTime    `json:"-"`
}

// Init implements the contracts.Initer interface.
func (h *HistoryAck) Init() {
	h.EventTime.History = h
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryAck) TableName() string {
	return "history"
}

type AckEventTime struct {
	History *HistoryAck `db:"-"`
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (et AckEventTime) Value() (driver.Value, error) {
	if et.History == nil {
		return nil, nil
	}

	switch et.History.EventType {
	case "ack_set":
		return et.History.SetTime.Value()
	case "ack_clear":
		return et.History.ClearTime.Value()
	default:
		return nil, nil
	}
}

// Assert interface compliance.
var (
	_ UpserterEntity       = (*AcknowledgementHistory)(nil)
	_ contracts.Initer     = (*HistoryAck)(nil)
	_ contracts.TableNamer = (*HistoryAck)(nil)
	_ UpserterEntity       = (*HistoryAck)(nil)
	_ driver.Valuer        = AckEventTime{}
)
