package eventcommandcustomvar

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

type EventCommandCustomvar struct {
	Id						string 		`json:"id"`
	EventCommandId			string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"env_id"`
}

func NewEventCommandCustomvar() connection.Row {
	c := EventCommandCustomvar{}
	return &c
}

func (c *EventCommandCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *EventCommandCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.EventCommandId),
		utils.Checksum(c.CustomvarId),
		utils.Checksum(c.EnvId),
	)

	return v
}

func (c *EventCommandCustomvar) GetId() string {
	return c.Id
}

func (c *EventCommandCustomvar) SetId(id string) {
	c.Id = id
}

func init() {
	name := "eventcommand_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "eventcommand:customvar",
		DeltaMySqlField: "id",
		Factory: NewEventCommandCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}