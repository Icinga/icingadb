package history

import (
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	icingadbTypes "github.com/icinga/icingadb/pkg/icingadb/types"
	"github.com/icinga/icingadb/pkg/types"
)

type CommentHistoryEntity struct {
	CommentId types.Binary `json:"comment_id"`
}

// Fingerprint implements part of the contracts.Entity interface.
func (che CommentHistoryEntity) Fingerprint() database.Fingerprinter {
	return che
}

// ID implements part of the contracts.Entity interface.
func (che CommentHistoryEntity) ID() database.ID {
	return che.CommentId
}

// SetID implements part of the contracts.Entity interface.
func (che *CommentHistoryEntity) SetID(id database.ID) {
	che.CommentId = id.(types.Binary)
}

type CommentHistoryUpserter struct {
	RemovedBy      types.String    `json:"removed_by"`
	RemoveTime     types.UnixMilli `json:"remove_time"`
	HasBeenRemoved types.Bool      `json:"has_been_removed"`
}

// Upsert implements the contracts.Upserter interface.
func (chu *CommentHistoryUpserter) Upsert() interface{} {
	return chu
}

type CommentHistory struct {
	CommentHistoryEntity   `json:",inline"`
	HistoryTableMeta       `json:",inline"`
	CommentHistoryUpserter `json:",inline"`
	EntryTime              types.UnixMilli           `json:"entry_time"`
	Author                 string                    `json:"author"`
	Comment                string                    `json:"comment"`
	EntryType              icingadbTypes.CommentType `json:"entry_type"`
	IsPersistent           types.Bool                `json:"is_persistent"`
	IsSticky               types.Bool                `json:"is_sticky"`
	ExpireTime             types.UnixMilli           `json:"expire_time"`
}

// Init implements the contracts.Initer interface.
func (ch *CommentHistory) Init() {
	ch.HasBeenRemoved = types.Bool{
		Bool:  false,
		Valid: true,
	}
}

type HistoryComment struct {
	HistoryMeta      `json:",inline"`
	CommentHistoryId types.Binary `json:"comment_id"`

	// Idea: read EntryTime, RemoveTime and ExpireTime from Redis
	// and let EventTime decide which of them to write to MySQL.
	// So EventTime doesn't have to be read from Redis (json:"-")
	// and the others don't have to be written to MySQL (db:"-").
	EntryTime  types.UnixMilli  `json:"entry_time" db:"-"`
	RemoveTime types.UnixMilli  `json:"remove_time" db:"-"`
	ExpireTime types.UnixMilli  `json:"expire_time" db:"-"`
	EventTime  CommentEventTime `json:"-"`
}

// Init implements the contracts.Initer interface.
func (h *HistoryComment) Init() {
	h.EventTime.History = h
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryComment) TableName() string {
	return "history"
}

type CommentEventTime struct {
	History *HistoryComment `db:"-"`
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (et CommentEventTime) Value() (driver.Value, error) {
	if et.History == nil {
		return nil, nil
	}

	switch et.History.EventType {
	case "comment_add":
		return et.History.EntryTime.Value()
	case "comment_remove":
		v, err := et.History.RemoveTime.Value()
		if err == nil && v == nil {
			return et.History.ExpireTime.Value()
		}

		return v, err
	default:
		return nil, nil
	}
}

// Assert interface compliance.
var (
	_ database.Entity     = (*CommentHistoryEntity)(nil)
	_ database.Upserter   = (*CommentHistoryUpserter)(nil)
	_ contracts.Initer    = (*CommentHistory)(nil)
	_ UpserterEntity      = (*CommentHistory)(nil)
	_ contracts.Initer    = (*HistoryComment)(nil)
	_ database.TableNamer = (*HistoryComment)(nil)
	_ UpserterEntity      = (*HistoryComment)(nil)
	_ driver.Valuer       = CommentEventTime{}
)
