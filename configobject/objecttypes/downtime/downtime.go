package downtime

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
		"object_type",
		"host_id",
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
		"is_flexible",
		"is_in_effect",
		"actual_start_time",
		"actual_end_time",
		"zone_id",
	}
)

type Downtime struct {
	Id                    string  	`json:"id"`
	EnvId                 string  	`json:"environment_id"`
	ObjectType            string  	`json:"object_type"`
	HostId                string  	`json:"host_id"`
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
	IsFlexible		      bool  	`json:"is_flexible"`
	IsInEffect		      bool  	`json:"is_in_effect"`
	ActualStartTime       float64  	`json:"actual_start_time"`
	ActualEndTime         float64  	`json:"actual_end_time"`
	ZoneId                string	`json:"zone_id"`
}

func NewDowntime() connection.Row {
	d := Downtime{}

	return &d
}

func (d *Downtime) InsertValues() []interface{} {
	v := d.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(d.Id)}, v...)
}

func (d *Downtime) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(d.EnvId),
		d.ObjectType,
		utils.EncodeChecksum(d.HostId),
		utils.EncodeChecksum(d.ServiceId),
		utils.EncodeChecksum(d.NameChecksum),
		utils.EncodeChecksum(d.PropertiesChecksum),
		d.Name,
		d.Author,
		d.Comment,
		d.EntryTime,
		d.ScheduledStartTime,
		d.ScheduledEndTime,
		d.Duration,
		utils.Bool[d.IsFlexible],
		utils.Bool[d.IsInEffect],
		d.ActualStartTime,
		d.ActualEndTime,
		utils.EncodeChecksum(d.ZoneId),
	)

	return v
}

func (d *Downtime) GetId() string {
	return d.Id
}

func (d *Downtime) SetId(id string) {
	d.Id = id
}

func (d *Downtime) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{d}, nil
}

func init() {
	name := "downtime"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "downtime",
		PrimaryMySqlField: "id",
		Factory: NewDowntime,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "downtime",
	}
}