package hoststate

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"host_id",
		"environment_id",
		"state_type",
		"soft_state",
		"hard_state",
		"attempt",
		"severity",
		"output",
		"long_output",
		"performance_data",
		"check_commandline",
		"is_problem",
		"is_handled",
		"is_reachable",
		"is_flapping",
		"is_acknowledged",
		"acknowledgement_comment_id",
		"in_downtime",
		"execution_time",
		"latency",
		"timeout",
		"last_update",
		"last_state_change",
		"last_soft_state",
		"last_hard_state",
		"next_check",
		"next_update",
	}
)

type HostState struct {
	HostId                  	string  `json:"id"`
	EnvId               		string  `json:"environment_id"`
	StateType					float32	`json:"state_type"`
	SoftState					float32	`json:"state"`
	HardState					float32	`json:"NONE"` //TODO (NoH): I'm not sure where to get this from
	Attempt						float32	`json:"check_attempt"`
	Severity					float32	`json:"severity"`
	Output						string	`json:"output"`
	LongOutput					string	`json:"long_output"`
	PerformanceData				string	`json:"performance_data"`
	CheckCommandline			string	`json:"commandline"`
	IsProblem					bool	`json:"is_problem"`
	IsHandled					bool	`json:"is_handled"`
	IsReachable					bool	`json:"is_reachable"`
	IsFlapping					bool	`json:"is_flapping"`
	IsAcknowledged				bool	`json:"is_acknowledged"`
	AcknowledgementCommentId 	string 	`json:"acknowledgement_comment_id"`
	InDowntime					bool	`json:"in_downtime"`
	ExecutionTime				float32	`json:"execution_time"`
	Latency						float32	`json:"latency"`
	Timeout						float32	`json:"check_timeout"`
	LastUpdate					float32	`json:"last_update"`
	LastStateChange				float32	`json:"last_state_change"`
	LastSoftState				float32	`json:"last_soft_state"`
	LastHardState				float32	`json:"last_hard_state"`
	NextCheck					float32	`json:"next_check"`
	NextUpdate					float32	`json:"next_update"`
}

func NewHostState() connection.Row {
	h := HostState{}

	return &h
}

func (h *HostState) InsertValues() []interface{} {
	v := h.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(h.HostId)}, v...)
}

func (h *HostState) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(h.EnvId),
		utils.IcingaStateTypeToString(h.StateType),
		h.SoftState,
		h.HardState,
		h.Attempt,
		h.Severity,
		h.Output,
		h.LongOutput,
		h.PerformanceData,
		h.CheckCommandline,
		utils.Bool[h.IsProblem],
		utils.Bool[h.IsHandled],
		utils.Bool[h.IsReachable],
		utils.Bool[h.IsFlapping],
		utils.Bool[h.IsAcknowledged],
		utils.EncodeChecksum(h.AcknowledgementCommentId),
		utils.Bool[h.InDowntime],
		h.ExecutionTime,
		h.Latency,
		h.Timeout,
		h.LastUpdate,
		h.LastStateChange,
		h.LastSoftState,
		h.LastHardState,
		h.NextCheck,
		h.NextUpdate,
	)

	return v
}

func (h *HostState) GetId() string {
	return h.HostId
}

func (h *HostState) SetId(id string) {
	h.HostId = id
}

func (h *HostState) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "host_state"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "state:host",
		PrimaryMySqlField: "host_id",
		Factory: NewHostState,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "host_id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "host",
	}
}