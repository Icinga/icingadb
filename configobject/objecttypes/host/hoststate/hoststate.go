// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package hoststate

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
	log "github.com/sirupsen/logrus"
	"time"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"host_id",
		"environment_id",
		"state_type",
		"soft_state",
		"hard_state",
		"previous_hard_state",
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
		"is_overdue",
		"is_acknowledged",
		"acknowledgement_comment_id",
		"in_downtime",
		"execution_time",
		"latency",
		"timeout",
		"check_source",
		"last_update",
		"last_state_change",
		"next_check",
		"next_update",
	}
)

type HostState struct {
	HostId                   string      `json:"id"`
	EnvId                    string      `json:"environment_id"`
	StateType                float32     `json:"state_type"`
	SoftState                float32     `json:"state"`
	HardState                float32     `json:"hard_state"`
	PreviousHardState        float32     `json:"previous_hard_state"`
	Attempt                  float32     `json:"check_attempt"`
	Severity                 float32     `json:"severity"`
	Output                   string      `json:"output"`
	LongOutput               string      `json:"long_output"`
	PerformanceData          string      `json:"performance_data"`
	CheckCommandline         string      `json:"commandline"`
	IsProblem                bool        `json:"is_problem"`
	IsHandled                bool        `json:"is_handled"`
	IsReachable              bool        `json:"is_reachable"`
	IsFlapping               bool        `json:"is_flapping"`
	Acknowledgement          float32     `json:"acknowledgement"`
	AcknowledgementCommentId string      `json:"acknowledgement_comment_id"`
	InDowntime               bool        `json:"in_downtime"`
	ExecutionTime            float64     `json:"execution_time"`
	Latency                  float64     `json:"latency"`
	Timeout                  float64     `json:"check_timeout"`
	CheckSource              string      `json:"check_source"`
	LastUpdate               interface{} `json:"last_update"`
	LastStateChange          float64     `json:"last_state_change"`
	NextCheck                float64     `json:"next_check"`
	NextUpdate               float64     `json:"next_update"`
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

	limit := 65535
    biglimit := 16777215
	outputVal, truncated := utils.TruncText(h.Output, biglimit)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "host_state",
			"Column": "output",
			"host_id": h.HostId,
		}).Infof("Truncated plugin output to 64KB")
	}

	longOutputVal, truncated := utils.TruncText(h.LongOutput, biglimit)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "host_state",
			"Column": "long_output",
			"host_id": h.HostId,
		}).Infof("Truncated long plugin output to 64KB")
	}

	perfData, truncated := utils.TruncPerfData(h.PerformanceData, biglimit)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "host_state",
			"Column": "performance_data",
			"host_id": h.HostId,
		}).Infof("Truncated plugin performance data to 64KB")
	}

	checkCmdLine, truncated := utils.TruncText(h.CheckCommandline, limit)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "host_state",
			"Column": "check_commandline",
			"host_id": h.HostId,
		}).Infof("Truncated plugin check command line to 64KB")
	}

	v = append(
		v,
		utils.EncodeChecksum(h.EnvId),
		utils.IcingaStateTypeToString(h.StateType),
		h.SoftState,
		h.HardState,
		h.PreviousHardState,
		h.Attempt,
		h.Severity,
		outputVal,
		longOutputVal,
		perfData,
		checkCmdLine,
		utils.Bool[h.IsProblem],
		utils.Bool[h.IsHandled],
		utils.Bool[h.IsReachable],
		utils.Bool[h.IsFlapping],
		utils.Bool[time.Now().After(utils.MillisecsToTime(float64(h.NextUpdate)))],
		utils.IsAcknowledged[h.Acknowledgement],
		utils.EncodeChecksumOrNil(h.AcknowledgementCommentId),
		utils.Bool[h.InDowntime],
		h.ExecutionTime,
		h.Latency,
		h.Timeout,
		h.CheckSource,
		h.LastUpdate,
		h.LastStateChange,
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
		ObjectType:               name,
		RedisKey:                 "state:host",
		PrimaryMySqlField:        "host_id",
		Factory:                  NewHostState,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "host_id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "host",
	}
}
