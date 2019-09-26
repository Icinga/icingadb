package usergroupcustomvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"usergroup_id",
		"customvar_id",
		"environment_id",
	}
)

type UsergroupCustomvar struct {
	Id						string 		`json:"id"`
	UsergroupId				string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"environment_id"`
}

func NewUsergroupCustomvar() connection.Row {
	c := UsergroupCustomvar{}
	return &c
}

func (c *UsergroupCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *UsergroupCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.UsergroupId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *UsergroupCustomvar) GetId() string {
	return c.Id
}

func (c *UsergroupCustomvar) SetId(id string) {
	c.Id = id
}

func (c *UsergroupCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "usergroup_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "usergroup:customvar",
		PrimaryMySqlField: "id",
		Factory: NewUsergroupCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "usergroup",
	}
}