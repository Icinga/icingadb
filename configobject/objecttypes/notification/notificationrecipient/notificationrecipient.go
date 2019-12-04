// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package notificationrecipient

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"notification_id",
		"user_id",
		"usergroup_id",
		"environment_id",
	}
)

type NotificationRecipient struct {
	Id             string      `json:"id"`
	NotificationId string      `json:"notification_id"`
	UserId         interface{} `json:"user_id"`
	UserGroupId    interface{} `json:"usergroup_id"`
	EnvId          string      `json:"environment_id"`
}

func NewNotificationRecipient() connection.Row {
	n := NotificationRecipient{}
	return &n
}

func (n *NotificationRecipient) InsertValues() []interface{} {
	v := n.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(n.Id)}, v...)
}

func (n *NotificationRecipient) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(n.NotificationId),
		utils.DecodeHexIfNotNil(n.UserId),
		utils.DecodeHexIfNotNil(n.UserGroupId),
		utils.EncodeChecksum(n.EnvId),
	)

	return v
}

func (n *NotificationRecipient) GetId() string {
	return n.Id
}

func (n *NotificationRecipient) SetId(id string) {
	n.Id = id
}

func (n *NotificationRecipient) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{n}, nil
}

func init() {
	name := "notification_recipient"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "notification:recipient",
		PrimaryMySqlField:        "id",
		Factory:                  NewNotificationRecipient,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notification",
	}
}
