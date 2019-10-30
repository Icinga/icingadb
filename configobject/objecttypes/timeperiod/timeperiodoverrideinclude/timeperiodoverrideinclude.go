package timeperiodoverrideinclude

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"timeperiod_id",
		"override_id",
		"environment_id",
	}
)

type TimeperiodOverrideInclude struct {
	Id           string `json:"id"`
	TimeperiodId string `json:"timeperiod_id"`
	OverrideId   string `json:"include_id"`
	EnvId        string `json:"environment_id"`
}

func NewTimeperiodOverrideInclude() connection.Row {
	t := TimeperiodOverrideInclude{}
	return &t
}

func (t *TimeperiodOverrideInclude) InsertValues() []interface{} {
	v := t.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(t.Id)}, v...)
}

func (t *TimeperiodOverrideInclude) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(t.TimeperiodId),
		utils.EncodeChecksum(t.OverrideId),
		utils.EncodeChecksum(t.EnvId),
	)

	return v
}

func (t *TimeperiodOverrideInclude) GetId() string {
	return t.Id
}

func (t *TimeperiodOverrideInclude) SetId(id string) {
	t.Id = id
}

func (t *TimeperiodOverrideInclude) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{t}, nil
}

func init() {
	name := "timeperiod_override_include"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "timeperiod:override:include",
		PrimaryMySqlField:        "id",
		Factory:                  NewTimeperiodOverrideInclude,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "timeperiod",
	}
}
