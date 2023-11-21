package v1

import (
	"github.com/icinga/icinga-go-library/types"
	icingadbTypes "github.com/icinga/icingadb/pkg/icingadb/types"
)

type State struct {
	EntityWithChecksum        `json:",inline"`
	EnvironmentMeta           `json:",inline"`
	AcknowledgementCommentId  types.Binary                       `json:"acknowledgement_comment_id"`
	LastCommentId             types.Binary                       `json:"last_comment_id"`
	CheckAttempt              uint8                              `json:"check_attempt"`
	CheckCommandline          types.String                       `json:"check_commandline"`
	CheckSource               types.String                       `json:"check_source"`
	SchedulingSource          types.String                       `json:"scheduling_source"`
	ExecutionTime             float64                            `json:"execution_time"`
	HardState                 uint8                              `json:"hard_state"`
	InDowntime                types.Bool                         `json:"in_downtime"`
	IsAcknowledged            icingadbTypes.AcknowledgementState `json:"is_acknowledged"`
	IsFlapping                types.Bool                         `json:"is_flapping"`
	IsHandled                 types.Bool                         `json:"is_handled"`
	IsProblem                 types.Bool                         `json:"is_problem"`
	IsReachable               types.Bool                         `json:"is_reachable"`
	LastStateChange           types.UnixMilli                    `json:"last_state_change"`
	LastUpdate                types.UnixMilli                    `json:"last_update"`
	Latency                   float64                            `json:"latency"`
	LongOutput                types.String                       `json:"long_output"`
	NextCheck                 types.UnixMilli                    `json:"next_check"`
	NextUpdate                types.UnixMilli                    `json:"next_update"`
	Output                    types.String                       `json:"output"`
	PerformanceData           types.String                       `json:"performance_data"`
	NormalizedPerformanceData types.String                       `json:"normalized_performance_data"`
	PreviousSoftState         uint8                              `json:"previous_soft_state"`
	PreviousHardState         uint8                              `json:"previous_hard_state"`
	Severity                  uint16                             `json:"severity"`
	SoftState                 uint8                              `json:"soft_state"`
	StateType                 icingadbTypes.StateType            `json:"state_type"`
	CheckTimeout              float64                            `json:"check_timeout"`
}
