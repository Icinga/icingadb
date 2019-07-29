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
		"env_id",
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
	EnvId                 string  	`json:"env_id"`
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

	return append([]interface{}{utils.Checksum(s.Id)}, v...)
}

func (s *ServiceDowntime) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(s.EnvId),
		utils.Checksum(s.ServiceId),
		utils.Checksum(s.NameChecksum),
		utils.Checksum(s.PropertiesChecksum),
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
		utils.Checksum(s.ZoneId),
	)

	return v
}

func (s *ServiceDowntime) GetId() string {
	return s.Id
}

func (s *ServiceDowntime) SetId(id string) {
	s.Id = id
}

func init() {
	name := "service_downtime"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "servicedowntime",
		Factory: NewServiceDowntime,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}