package history

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type StateHistory struct {
	HistoryTableEntity `json:",inline"`
	HistoryTableMeta   `json:",inline"`
	EventTime          types.UnixMilli `json:"event_time"`
	StateType          types.StateType `json:"state_type"`
	SoftState          uint8           `json:"soft_state"`
	HardState          uint8           `json:"hard_state"`
	PreviousSoftState  uint8           `json:"previous_soft_state"`
	PreviousHardState  uint8           `json:"previous_hard_state"`
	Attempt            uint8           `json:"attempt"`
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

// Assert interface compliance.
var (
	_ UpserterEntity       = (*StateHistory)(nil)
	_ contracts.TableNamer = (*HistoryState)(nil)
	_ UpserterEntity       = (*HistoryState)(nil)
)
