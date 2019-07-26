package notificationcommandenvvar

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

type NotificationCommandEnvvar struct {
	Id						string 		`json:"id"`
	CommandId				string		`json:"command_id"`
	EnvvarKey				string		`json:"envvar_key"`
	EnvId           		string		`json:"env_id"`
	PropertiesChecksum		string		`json:"checksum"`
	EnvvarValue				string		`json:"value"`
}

func NewNotificationCommandEnvvar() connection.Row {
	c := NotificationCommandEnvvar{}
	return &c
}

func (c *NotificationCommandEnvvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *NotificationCommandEnvvar) UpdateValues() []interface{} {
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

func (c *NotificationCommandEnvvar) GetId() string {
	return c.Id
}

func (c *NotificationCommandEnvvar) SetId(id string) {
	c.Id = id
}

func init() {
	name := "notificationcommand_envvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "notificationcommand:envvar",
		Factory: NewNotificationCommandEnvvar,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}