package notificationcustomvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"notification_id",
		"customvar_id",
		"environment_id",
	}
)

type NotificationCustomvar struct {
	Id             string `json:"id"`
	NotificationId string `json:"object_id"`
	CustomvarId    string `json:"customvar_id"`
	EnvId          string `json:"environment_id"`
}

func NewNotificationCustomvar() connection.Row {
	c := NotificationCustomvar{}
	return &c
}

func (c *NotificationCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *NotificationCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.NotificationId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *NotificationCustomvar) GetId() string {
	return c.Id
}

func (c *NotificationCustomvar) SetId(id string) {
	c.Id = id
}

func (c *NotificationCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "notification_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "notification:customvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewNotificationCustomvar,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notification",
	}
}
