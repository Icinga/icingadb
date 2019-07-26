package notificationcommandcustomvar

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

type NotificationCommandCustomvar struct {
	Id						string 		`json:"id"`
	NotificationCommandId	string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"env_id"`
}

func NewNotificationCommandCustomvar() connection.Row {
	c := NotificationCommandCustomvar{}
	return &c
}

func (c *NotificationCommandCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *NotificationCommandCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.NotificationCommandId),
		utils.Checksum(c.CustomvarId),
		utils.Checksum(c.EnvId),
	)

	return v
}

func (c *NotificationCommandCustomvar) GetId() string {
	return c.Id
}

func (c *NotificationCommandCustomvar) SetId(id string) {
	c.Id = id
}

func init() {
	name := "notificationcommand_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "notificationcommand:customvar",
		Factory: NewNotificationCommandCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}