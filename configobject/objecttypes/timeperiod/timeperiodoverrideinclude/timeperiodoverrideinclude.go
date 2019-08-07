package timeperiodoverrideinclude

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"timeperiod_id",
		"override_id",
		"env_id",
	}
)

type TimeperiodOverrideInclude struct {
	Id						string 		`json:"id"`
	TimeperiodId			string		`json:"timeperiod_id"`
	OverrideId	 			string 		`json:"include_id"`
	EnvId           		string		`json:"env_id"`
}

func NewTimeperiodOverrideInclude() connection.Row {
	t := TimeperiodOverrideInclude{}
	return &t
}

func (t *TimeperiodOverrideInclude) InsertValues() []interface{} {
	v := t.UpdateValues()

	return append([]interface{}{utils.Checksum(t.Id)}, v...)
}

func (t *TimeperiodOverrideInclude) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(t.TimeperiodId),
		utils.Checksum(t.OverrideId),
		utils.Checksum(t.EnvId),
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
		ObjectType: name,
		RedisKey: "timeperiod:override:include",
		DeltaMySqlField: "id",
		Factory: NewTimeperiodOverrideInclude,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}