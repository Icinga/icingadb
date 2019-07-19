package notificationuser

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"notification_id",
		"user_id",
		"env_id",
	}
)

type NotificationUser struct {
	Id						string 		`json:"id"`
	NotificationId			string		`json:"notification_id"`
	UserId	 				string 		`json:"user_id"`
	EnvId           		string		`json:"env_id"`
}

func NewNotificationUser() connection.Row {
	n := NotificationUser{}
	return &n
}

func (n *NotificationUser) InsertValues() []interface{} {
	v := n.UpdateValues()

	return append([]interface{}{utils.Checksum(n.Id)}, v...)
}

func (n *NotificationUser) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(n.NotificationId),
		utils.Checksum(n.UserId),
		utils.Checksum(n.EnvId),
	)

	return v
}

func (n *NotificationUser) GetId() string {
	return n.Id
}

func (n *NotificationUser) SetId(id string) {
	n.Id = id
}

func init() {
	name := "notification_user"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "notification:user",
		Factory: NewNotificationUser,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}