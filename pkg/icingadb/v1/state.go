package v1

import (
	"github.com/icinga/icingadb/pkg/types"
)

type State struct {
	EntityWithChecksum       `json:",inline"`
	EnvironmentMeta          `json:",inline"`
	AcknowledgementCommentId types.Binary               `json:"acknowledgement_comment_id"`
	Attempt                  uint8                      `json:"check_attempt"`
	CheckCommandline         types.Text                 `json:"commandline"`
	CheckSource              types.String               `json:"check_source"`
	ExecutionTime            float64                    `json:"execution_time"`
	HardState                uint8                      `json:"hard_state"`
	InDowntime               types.Bool                 `json:"in_downtime"`
	IsAcknowledged           types.AcknowledgementState `json:"acknowledgement"`
	IsFlapping               types.Bool                 `json:"is_flapping"`
	IsHandled                types.Bool                 `json:"is_handled"`
	IsProblem                types.Bool                 `json:"is_problem"`
	IsReachable              types.Bool                 `json:"is_reachable"`
	LastStateChange          types.UnixMilli            `json:"last_state_change"`
	LastUpdate               types.UnixMilli            `json:"last_update"`
	Latency                  float64                    `json:"latency"`
	LongOutput               types.Output               `json:"long_output"`
	NextCheck                types.UnixMilli            `json:"next_check"`
	NextUpdate               types.UnixMilli            `json:"next_update"`
	Output                   types.Output               `json:"output"`
	PerformanceData          types.PerfData             `json:"performance_data"`
	PreviousHardState        uint8                      `json:"previous_hard_state"`
	Severity                 uint16                     `json:"severity"`
	SoftState                uint8                      `json:"state"`
	StateType                types.StateType            `json:"state_type"`
	Timeout                  float64                    `json:"check_timeout"`
}
