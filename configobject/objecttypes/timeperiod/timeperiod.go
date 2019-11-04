package timeperiod

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"name",
		"name_ci",
		"display_name",
		"prefer_includes",
		"zone_id",
	}
)

type Timeperiod struct {
	Id                 string  `json:"id"`
	EnvId              string  `json:"environment_id"`
	NameChecksum       string  `json:"name_checksum"`
	PropertiesChecksum string  `json:"checksum"`
	Name               string  `json:"name"`
	NameCi             *string `json:"name_ci"`
	DisplayName        string  `json:"display_name"`
	PreferIncludes     bool    `json:"prefer_includes"`
	ZoneId             string  `json:"zone_id"`
}

func NewTimeperiod() connection.Row {
	t := Timeperiod{}
	t.NameCi = &t.Name

	return &t
}

func (t *Timeperiod) InsertValues() []interface{} {
	v := t.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(t.Id)}, v...)
}

func (t *Timeperiod) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(t.EnvId),
		utils.EncodeChecksum(t.NameChecksum),
		utils.EncodeChecksum(t.PropertiesChecksum),
		t.Name,
		t.NameCi,
		t.DisplayName,
		utils.Bool[t.PreferIncludes],
		utils.EncodeChecksum(t.ZoneId),
	)

	return v
}

func (t *Timeperiod) GetId() string {
	return t.Id
}

func (t *Timeperiod) SetId(id string) {
	t.Id = id
}

func (t *Timeperiod) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{t}, nil
}

func init() {
	name := "timeperiod"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewTimeperiod,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "timeperiod",
	}
}
