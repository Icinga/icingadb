package checkcommandcustomvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"command_id",
		"customvar_id",
		"env_id",
	}
)

type CheckCommandCustomvar struct {
	Id						string 		`json:"id"`
	CheckCommandId			string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"env_id"`
}

func NewCheckCommandCustomvar() connection.Row {
	c := CheckCommandCustomvar{}
	return &c
}

func (c *CheckCommandCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *CheckCommandCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.CheckCommandId),
		utils.Checksum(c.CustomvarId),
		utils.Checksum(c.EnvId),
	)

	return v
}

func (c *CheckCommandCustomvar) GetId() string {
	return c.Id
}

func (c *CheckCommandCustomvar) SetId(id string) {
	c.Id = id
}

func init() {
	name := "checkcommand_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "checkcommand:customvar",
		DeltaMySqlField: "id",
		Factory: NewCheckCommandCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}