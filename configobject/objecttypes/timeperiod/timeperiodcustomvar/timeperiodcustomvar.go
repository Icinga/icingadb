package timeperiodcustomvar

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
		"customvar_id",
		"environment_id",
	}
)

type TimeperiodCustomvar struct {
	Id						string 		`json:"id"`
	TimeperiodId			string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"environment_id"`
}

func NewTimeperiodCustomvar() connection.Row {
	c := TimeperiodCustomvar{}
	return &c
}

func (c *TimeperiodCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *TimeperiodCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.TimeperiodId),
		utils.Checksum(c.CustomvarId),
		utils.Checksum(c.EnvId),
	)

	return v
}

func (c *TimeperiodCustomvar) GetId() string {
	return c.Id
}

func (c *TimeperiodCustomvar) SetId(id string) {
	c.Id = id
}

func (c *TimeperiodCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "timeperiod_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "timeperiod:customvar",
		PrimaryMySqlField: "id",
		Factory: NewTimeperiodCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "timeperiod",
	}
}