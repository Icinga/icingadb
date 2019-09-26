package servicestate

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"service_id",
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
	}
)

type ServiceState struct {
	ServiceId                  	string  `json:"id"`
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
}

func NewServiceState() connection.Row {
	s := ServiceState{}

	return &s
}

func (s *ServiceState) InsertValues() []interface{} {
	v := s.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(s.ServiceId)}, v...)
}

func (s *ServiceState) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(s.EnvId),
		utils.IcingaStateTypeToString(s.StateType),
		s.SoftState,
		s.HardState,
		s.Attempt,
		s.Severity,
		s.Output,
		s.LongOutput,
		s.PerformanceData,
		s.CheckCommandline,
		utils.Bool[s.IsProblem],
		utils.Bool[s.IsHandled],
		utils.Bool[s.IsReachable],
		utils.Bool[s.IsFlapping],
		utils.Bool[s.IsAcknowledged],
		utils.EncodeChecksum(s.AcknowledgementCommentId),
		utils.Bool[s.InDowntime],
		s.ExecutionTime,
		s.Latency,
		s.Timeout,
		s.LastUpdate,
		s.LastStateChange,
		s.LastSoftState,
		s.LastHardState,
		s.NextCheck,
	)

	return v
}

func (s *ServiceState) GetId() string {
	return s.ServiceId
}

func (s *ServiceState) SetId(id string) {
	s.ServiceId = id
}

func (s *ServiceState) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{s}, nil
}

func init() {
	name := "service_state"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "state:service",
		PrimaryMySqlField: "service_id",
		Factory: NewServiceState,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "service_id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "service_id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "service",
	}
}