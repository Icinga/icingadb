package v1

import (
	"encoding/json"

	"github.com/icinga/icinga-go-library/types"
)

type State struct {
	EntityWithChecksum        `json:",inline"`
	EnvironmentMeta           `json:",inline"`
	AcknowledgementCommentId  types.Binary    `json:"acknowledgement_comment_id"`
	LastCommentId             types.Binary    `json:"last_comment_id"`
	CheckAttempt              uint32          `json:"check_attempt"`
	CheckCommandline          types.String    `json:"check_commandline"`
	CheckSource               types.String    `json:"check_source"`
	SchedulingSource          types.String    `json:"scheduling_source"`
	ExecutionTime             types.Float     `json:"execution_time"`
	HardState                 uint8           `json:"hard_state"`
	InDowntime                types.Bool      `json:"in_downtime"`
	AffectsChildren           types.Bool      `json:"affects_children"`
	IsAcknowledged            types.Bool      `json:"is_acknowledged"`
	IsStickyAcknowledgement   types.Bool      `json:"is_sticky_acknowledgement"`
	AcknowledgementSetTime    types.UnixMilli `json:"acknowledgement_set_time"`
	IsFlapping                types.Bool      `json:"is_flapping"`
	IsHandled                 types.Bool      `json:"is_handled"`
	IsProblem                 types.Bool      `json:"is_problem"`
	IsReachable               types.Bool      `json:"is_reachable"`
	LastStateChange           types.UnixMilli `json:"last_state_change"`
	LastUpdate                types.UnixMilli `json:"last_update"`
	Latency                   types.Float     `json:"latency"`
	LongOutput                types.String    `json:"long_output"`
	NextCheck                 types.UnixMilli `json:"next_check"`
	NextUpdate                types.UnixMilli `json:"next_update"`
	Output                    types.String    `json:"output"`
	PerformanceData           types.String    `json:"performance_data"`
	NormalizedPerformanceData types.String    `json:"normalized_performance_data"`
	PreviousSoftState         uint8           `json:"previous_soft_state"`
	PreviousHardState         uint8           `json:"previous_hard_state"`
	Severity                  uint16          `json:"severity"`
	SoftState                 uint8           `json:"soft_state"`
	StateType                 string          `json:"state_type"`
	CheckTimeout              types.Float     `json:"check_timeout"`

	MetaData EphemeralStateInfo `json:"metadata" db:"-"` // Excluded from database storage.
}

// EphemeralStateInfo contains additional metadata about the state that is not stored in the database.
//
// This struct is used to provide additional context about the state, which is going to be used only in memory
// and not persisted in the database. Currently, these fields are mainly used by the Icinga Notifications component.
type EphemeralStateInfo struct {
	StateChange            types.Bool      `json:"is_state_change"`
	ExecutionEnd           types.UnixMilli `json:"execution_end"`
	DowntimeTransitionType types.String    `json:"downtime_transition_type"`
	AckTransitionType      types.String    `json:"ack_transition_type"`

	LastTriggeredDowntimeName types.String `json:"last_triggered_downtime_name"`
	LastRemovedDowntimeName   types.String `json:"last_removed_downtime_name"`
}

// IsStateChange returns true if the state is a state change, false otherwise.
func (esi *EphemeralStateInfo) IsStateChange() bool {
	return esi.StateChange.Valid && esi.StateChange.Bool
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface for EphemeralStateInfo.
func (esi *EphemeralStateInfo) UnmarshalText(text []byte) error { return esi.UnmarshalJSON(text) }

// UnmarshalJSON implements the [json.Unmarshaler] interface for EphemeralStateInfo.
func (esi *EphemeralStateInfo) UnmarshalJSON(data []byte) error {
	// Use an alias to unmarshal the JSON data, otherwise, it would cause infinite recursion (might OOM or stack overflow).
	type alias EphemeralStateInfo
	return json.Unmarshal(data, (*alias)(esi))
}
