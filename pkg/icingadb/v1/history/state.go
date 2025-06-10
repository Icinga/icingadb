package history

import (
	"database/sql/driver"
	"encoding"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/common"
)

type StateHistory struct {
	HistoryTableEntity `json:",inline"`
	HistoryTableMeta   `json:",inline"`
	EventTime          types.UnixMilli `json:"event_time"`
	StateType          StateType       `json:"state_type"`
	SoftState          uint8           `json:"soft_state"`
	HardState          uint8           `json:"hard_state"`
	PreviousSoftState  uint8           `json:"previous_soft_state"`
	PreviousHardState  uint8           `json:"previous_hard_state"`
	CheckAttempt       uint32          `json:"check_attempt"`
	Output             types.String    `json:"output"`
	LongOutput         types.String    `json:"long_output"`
	MaxCheckAttempts   uint32          `json:"max_check_attempts"`
	CheckSource        types.String    `json:"check_source"`
	SchedulingSource   types.String    `json:"scheduling_source"`
}

type HistoryState struct {
	HistoryMeta    `json:",inline"`
	StateHistoryId types.Binary    `json:"id"`
	EventTime      types.UnixMilli `json:"event_time"`
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryState) TableName() string {
	return "history"
}

type SlaHistoryState struct {
	HistoryTableEntity `json:",inline"`
	HistoryTableMeta   `json:",inline"`
	EventTime          types.UnixMilli `json:"event_time"`
	StateType          StateType       `json:"state_type" db:"-"`
	HardState          uint8           `json:"hard_state"`
	PreviousHardState  uint8           `json:"previous_hard_state"`
}

// StateType represents the type of state for a state history entry.
//
// Starting with Icinga 2 v2.15, the type is will always be written to Redis as a string.
// This merely exists to provide compatibility with older history entries lying around in Redis,
// which may have been written as their integer representation (0, 1) which stands for soft and hard state.
type StateType string

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (st *StateType) UnmarshalText(text []byte) error {
	switch t := string(text); t {
	case "0":
		*st = common.SoftState
	case "1":
		*st = common.HardState
	default:
		*st = StateType(t)
	}

	return nil
}

// Value implements the driver.Valuer interface.
func (st StateType) Value() (driver.Value, error) { return string(st), nil }

// Assert interface compliance.
var (
	_ UpserterEntity           = (*StateHistory)(nil)
	_ database.TableNamer      = (*HistoryState)(nil)
	_ UpserterEntity           = (*HistoryState)(nil)
	_ UpserterEntity           = (*SlaHistoryState)(nil)
	_ encoding.TextUnmarshaler = (*StateType)(nil)
	_ driver.Valuer            = StateType("")
)
