package notificationcommandcustomvar

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"command_id",
		"customvar_id",
		"environment_id",
	}
)

type NotificationCommandCustomvar struct {
	Id                    string `json:"id"`
	NotificationCommandId string `json:"object_id"`
	CustomvarId           string `json:"customvar_id"`
	EnvId                 string `json:"environment_id"`
}

func NewNotificationCommandCustomvar() connection.Row {
	c := NotificationCommandCustomvar{}
	return &c
}

func (c *NotificationCommandCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *NotificationCommandCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.NotificationCommandId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *NotificationCommandCustomvar) GetId() string {
	return c.Id
}

func (c *NotificationCommandCustomvar) SetId(id string) {
	c.Id = id
}

func (c *NotificationCommandCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "notificationcommand_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "notificationcommand:customvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewNotificationCommandCustomvar,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notificationcommand",
	}
}
