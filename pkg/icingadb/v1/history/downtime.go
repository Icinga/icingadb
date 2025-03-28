package history

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/contracts"
	"golang.org/x/sync/errgroup"
	"time"
)

type DowntimeHistoryEntity struct {
	DowntimeId types.Binary `json:"downtime_id"`
}

// Fingerprint implements part of the contracts.Entity interface.
func (dhe DowntimeHistoryEntity) Fingerprint() database.Fingerprinter {
	return dhe
}

// ID implements part of the contracts.Entity interface.
func (dhe DowntimeHistoryEntity) ID() database.ID {
	return dhe.DowntimeId
}

// SetID implements part of the contracts.Entity interface.
//
// The id must be of type types.Binary. Otherwise, the method will panic.
func (dhe *DowntimeHistoryEntity) SetID(id database.ID) {
	idBinary, ok := id.(types.Binary)
	if !ok {
		panic(fmt.Sprintf("expects types.Binary, got %T", id))
	}
	dhe.DowntimeId = idBinary
}

type DowntimeHistoryUpserter struct {
	CancelledBy      types.String    `json:"cancelled_by"`
	HasBeenCancelled types.Bool      `json:"has_been_cancelled"`
	CancelTime       types.UnixMilli `json:"cancel_time"`
}

// Upsert implements the contracts.Upserter interface.
func (dhu *DowntimeHistoryUpserter) Upsert() any {
	return dhu
}

type DowntimeHistory struct {
	DowntimeHistoryEntity   `json:",inline"`
	HistoryTableMeta        `json:",inline"`
	DowntimeHistoryUpserter `json:",inline"`
	TriggeredById           types.Binary    `json:"triggered_by_id"`
	ParentId                types.Binary    `json:"parent_id"`
	EntryTime               types.UnixMilli `json:"entry_time"`
	Author                  string          `json:"author"`
	Comment                 string          `json:"comment"`
	IsFlexible              types.Bool      `json:"is_flexible"`
	FlexibleDuration        uint64          `json:"flexible_duration"`
	ScheduledStartTime      types.UnixMilli `json:"scheduled_start_time"`
	ScheduledEndTime        types.UnixMilli `json:"scheduled_end_time"`
	StartTime               types.UnixMilli `json:"start_time"`
	EndTime                 types.UnixMilli `json:"end_time"`
	ScheduledBy             types.String    `json:"scheduled_by"`
	TriggerTime             types.UnixMilli `json:"trigger_time"`
}

type HistoryDowntime struct {
	HistoryMeta       `json:",inline"`
	DowntimeHistoryId types.Binary `json:"downtime_id"`

	// Idea: read StartTime, CancelTime, EndTime and HasBeenCancelled from Redis
	// and let EventTime decide based on HasBeenCancelled which of the others to write to MySQL.
	// So EventTime doesn't have to be read from Redis (json:"-")
	// and the others don't have to be written to MySQL (db:"-").
	StartTime        types.UnixMilli   `json:"start_time" db:"-"`
	CancelTime       types.UnixMilli   `json:"cancel_time" db:"-"`
	EndTime          types.UnixMilli   `json:"end_time" db:"-"`
	HasBeenCancelled types.Bool        `json:"has_been_cancelled" db:"-"`
	EventTime        DowntimeEventTime `json:"-"`
}

// Init implements the contracts.Initer interface.
func (h *HistoryDowntime) Init() {
	h.EventTime.History = h
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryDowntime) TableName() string {
	return "history"
}

// DowntimeHistoryMeta is a combined HistoryMeta struct of DowntimeHistoryEntity and DowntimeHistory.
//
// It is used in the notifications package and became necessary as values of both structs were required.
type DowntimeHistoryMeta struct {
	DowntimeHistoryEntity `json:",inline"`
	DowntimeHistory       `json:",inline"`
	HistoryMeta           `json:",inline"`
}

type SlaHistoryDowntime struct {
	DowntimeHistoryEntity      `json:",inline"`
	HistoryTableMeta           `json:",inline"`
	SlaHistoryDowntimeUpserter `json:",inline"`
	DowntimeStart              types.UnixMilli `json:"start_time"`
	HasBeenCancelled           types.Bool      `json:"has_been_cancelled" db:"-"`
	CancelTime                 types.UnixMilli `json:"cancel_time" db:"-"`
	EndTime                    types.UnixMilli `json:"end_time" db:"-"`
}

// Init implements the contracts.Initer interface.
func (s *SlaHistoryDowntime) Init() {
	s.DowntimeEnd.History = s
}

type SlaHistoryDowntimeUpserter struct {
	DowntimeEnd SlaDowntimeEndTime `json:"-"`
}

// Upsert implements the contracts.Upserter interface.
func (h *SlaHistoryDowntimeUpserter) Upsert() any {
	return h
}

type DowntimeEventTime struct {
	History *HistoryDowntime `db:"-"`
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (et DowntimeEventTime) Value() (driver.Value, error) {
	if et.History == nil {
		return nil, nil
	}

	switch et.History.EventType {
	case "downtime_start":
		return et.History.StartTime.Value()
	case "downtime_end":
		if !et.History.HasBeenCancelled.Valid {
			return nil, nil
		}

		if et.History.HasBeenCancelled.Bool {
			return et.History.CancelTime.Value()
		} else {
			return et.History.EndTime.Value()
		}
	default:
		return nil, nil
	}
}

type SlaDowntimeEndTime struct {
	History *SlaHistoryDowntime `db:"-"`
}

// Value implements the driver.Valuer interface.
func (et SlaDowntimeEndTime) Value() (driver.Value, error) {
	if et.History.HasBeenCancelled.Valid && et.History.HasBeenCancelled.Bool {
		return et.History.CancelTime.Value()
	} else {
		return et.History.EndTime.Value()
	}
}

// SyncDowntimeHistoryEndEvent updates the downtime history records with the given downtime IDs to mark them as cancelled.
//
// This function is used to mark downtimes as cancelled when the downtime configuration is removed from
// the configuration files. In such cases, Icinga 2 won't send the corresponding downtime end/removed events,
// so we need to mark the downtimes as cancelled manually.
func SyncDowntimeHistoryEndEvent(ctx context.Context, db *database.DB, downtimeIds []any) error {
	downtimes := make(chan database.Entity, len(downtimeIds))
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(downtimes)

		// Transform the downtime IDs into DowntimeHistory entities with the necessary fields
		// and stream them into the downtimes channel.
		for _, id := range downtimeIds {
			// The downtimes channel is buffered, so this will never block, and we can just keep sending.
			downtimes <- &DowntimeHistory{
				DowntimeHistoryEntity: DowntimeHistoryEntity{DowntimeId: id.(types.Binary)},
				DowntimeHistoryUpserter: DowntimeHistoryUpserter{
					CancelledBy:      types.MakeString("Downtime Config Removed"),
					HasBeenCancelled: types.Bool{Bool: true, Valid: true},
					CancelTime:       types.UnixMilli(time.Now()),
				},
			}
		}

		return nil
	})

	g.Go(func() error {
		// Each downtime removed in this manner should never have been cancelled before, so we
		// don't need extra where clauses in the update statement other than the downtime ID.
		stmt := `UPDATE downtime_history SET cancel_time = :cancel_time, has_been_cancelled = :has_been_cancelled, cancelled_by = :cancelled_by WHERE downtime_id = :downtime_id`

		return db.NamedBulkExecTx(ctx, stmt, db.Options.MaxRowsPerTransaction, db.GetSemaphoreForTable("downtime_history"), downtimes)
	})

	// TODO: Fake the downtime end event in the regular history and SLA downtime history tables.

	return g.Wait()
}

// Assert interface compliance.
var (
	_ database.Entity     = (*DowntimeHistoryEntity)(nil)
	_ database.Upserter   = (*DowntimeHistoryUpserter)(nil)
	_ UpserterEntity      = (*DowntimeHistory)(nil)
	_ contracts.Initer    = (*HistoryDowntime)(nil)
	_ database.TableNamer = (*HistoryDowntime)(nil)
	_ UpserterEntity      = (*HistoryDowntime)(nil)
	_ contracts.Initer    = (*SlaHistoryDowntime)(nil)
	_ UpserterEntity      = (*SlaHistoryDowntime)(nil)
	_ driver.Valuer       = DowntimeEventTime{}
	_ driver.Valuer       = SlaDowntimeEndTime{}
)
