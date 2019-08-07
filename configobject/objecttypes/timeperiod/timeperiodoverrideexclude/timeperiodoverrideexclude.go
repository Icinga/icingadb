package timeperiodoverrideexclude

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

type TimeperiodOverrideExclude struct {
	Id						string 		`json:"id"`
	TimeperiodId			string		`json:"timeperiod_id"`
	OverrideId	 			string 		`json:"exclude_id"`
	EnvId           		string		`json:"env_id"`
}

func NewTimeperiodOverrideExclude() connection.Row {
	t := TimeperiodOverrideExclude{}
	return &t
}

func (t *TimeperiodOverrideExclude) InsertValues() []interface{} {
	v := t.UpdateValues()

	return append([]interface{}{utils.Checksum(t.Id)}, v...)
}

func (t *TimeperiodOverrideExclude) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(t.TimeperiodId),
		utils.Checksum(t.OverrideId),
		utils.Checksum(t.EnvId),
	)

	return v
}

func (t *TimeperiodOverrideExclude) GetId() string {
	return t.Id
}

func (t *TimeperiodOverrideExclude) SetId(id string) {
	t.Id = id
}

func (t *TimeperiodOverrideExclude) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{t}, nil
}

func init() {
	name := "timeperiod_override_exclude"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "timeperiod:override:exclude",
		DeltaMySqlField: "id",
		Factory: NewTimeperiodOverrideExclude,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}