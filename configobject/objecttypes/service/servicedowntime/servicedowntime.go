package servicedowntime

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
		"service_id",
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

type ServiceDowntime struct {
	Id                    string  	`json:"id"`
	EnvId                 string  	`json:"environment_id"`
	ServiceId             string  	`json:"service_id"`
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

func NewServiceDowntime() connection.Row {
	s := ServiceDowntime{}

	return &s
}

func (s *ServiceDowntime) InsertValues() []interface{} {
	v := s.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(s.Id)}, v...)
}

func (s *ServiceDowntime) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(s.EnvId),
		utils.EncodeChecksum(s.ServiceId),
		utils.EncodeChecksum(s.NameChecksum),
		utils.EncodeChecksum(s.PropertiesChecksum),
		s.Name,
		s.Author,
		s.Comment,
		s.EntryTime,
		s.ScheduledStartTime,
		s.ScheduledEndTime,
		s.Duration,
		utils.Bool[s.IsFixed],
		utils.Bool[s.IsInEffect],
		s.ActualStartTime,
		s.ActualEndTime,
		utils.EncodeChecksum(s.ZoneId),
	)

	return v
}

func (s *ServiceDowntime) GetId() string {
	return s.Id
}

func (s *ServiceDowntime) SetId(id string) {
	s.Id = id
}

func (s *ServiceDowntime) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{s}, nil
}

func init() {
	name := "service_downtime"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "servicedowntime",
		PrimaryMySqlField: "id",
		Factory: NewServiceDowntime,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "servicedowntime",
	}
}