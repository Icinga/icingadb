package hostdowntime

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"environment_id",
		"host_id",
		"name_checksum",
		"properties_checksum",
		"name",
		"author",
		"comment",
		"entry_time",
		"scheduled_start_time",
		"scheduled_end_time",
		"duration",
		"is_fixed",
		"is_in_effect",
		"actual_start_time",
		"actual_end_time",
		"zone_id",
	}
)

type HostDowntime struct {
	Id                    string  	`json:"id"`
	EnvId                 string  	`json:"environment_id"`
	HostId                string  	`json:"host_id"`
	NameChecksum          string  	`json:"name_checksum"`
	PropertiesChecksum    string  	`json:"checksum"`
	Name                  string  	`json:"name"`
	Author                string  	`json:"author"`
	Comment               string  	`json:"comment"`
	EntryTime             float64  	`json:"entry_time"`
	ScheduledStartTime    float64  	`json:"scheduled_start_time"`
	ScheduledEndTime      float64  	`json:"scheduled_end_time"`
	Duration		      float64  	`json:"duration"`
	IsFixed		      	  bool  	`json:"is_fixed"`
	IsInEffect		      bool  	`json:"is_in_effect"`
	ActualStartTime       float64  	`json:"actual_start_time"`
	ActualEndTime         float64  	`json:"actual_end_time"`
	ZoneId                string	`json:"zone_id"`
}

func NewHostDowntime() connection.Row {
	h := HostDowntime{}

	return &h
}

func (h *HostDowntime) InsertValues() []interface{} {
	v := h.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(h.Id)}, v...)
}

func (h *HostDowntime) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(h.EnvId),
		utils.EncodeChecksum(h.HostId),
		utils.EncodeChecksum(h.NameChecksum),
		utils.EncodeChecksum(h.PropertiesChecksum),
		h.Name,
		h.Author,
		h.Comment,
		h.EntryTime,
		h.ScheduledStartTime,
		h.ScheduledEndTime,
		h.Duration,
		utils.Bool[h.IsFixed],
		utils.Bool[h.IsInEffect],
		h.ActualStartTime,
		h.ActualEndTime,
		utils.EncodeChecksum(h.ZoneId),
	)

	return v
}

func (h *HostDowntime) GetId() string {
	return h.Id
}

func (h *HostDowntime) SetId(id string) {
	h.Id = id
}

func (h *HostDowntime) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "host_downtime"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "hostdowntime",
		PrimaryMySqlField: "id",
		Factory: NewHostDowntime,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "hostdowntime",
	}
}