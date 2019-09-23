package eventcommandenvvar

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
		"envvar_key",
		"env_id",
		"properties_checksum",
		"envvar_value",
	}
)

type EventCommandEnvvar struct {
	Id						string 		`json:"id"`
	CommandId				string		`json:"command_id"`
	EnvvarKey				string		`json:"envvar_key"`
	EnvId           		string		`json:"env_id"`
	PropertiesChecksum		string		`json:"checksum"`
	EnvvarValue				string		`json:"value"`
}

func NewEventCommandEnvvar() connection.Row {
	c := EventCommandEnvvar{}
	return &c
}

func (c *EventCommandEnvvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *EventCommandEnvvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.CommandId),
		c.EnvvarKey,
		utils.Checksum(c.EnvId),
		utils.Checksum(c.PropertiesChecksum),
		c.EnvvarValue,
	)

	return v
}

func (c *EventCommandEnvvar) GetId() string {
	return c.Id
}

func (c *EventCommandEnvvar) SetId(id string) {
	c.Id = id
}

func (c *EventCommandEnvvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "eventcommand_envvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "eventcommand:envvar",
		PrimaryMySqlField: "id",
		Factory: NewEventCommandEnvvar,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "eventcommand",
	}
}