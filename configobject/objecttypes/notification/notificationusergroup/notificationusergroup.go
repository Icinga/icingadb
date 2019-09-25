package notificationusergroup

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
		"usergroup_id",
		"environment_id",
	}
)

type NotificationUsergroup struct {
	Id						string 		`json:"id"`
	NotificationId			string		`json:"notification_id"`
	UsergroupId	 			string 		`json:"usergroup_id"`
	EnvId           		string		`json:"environment_id"`
}

func NewNotificationUsergroup() connection.Row {
	n := NotificationUsergroup{}
	return &n
}

func (n *NotificationUsergroup) InsertValues() []interface{} {
	v := n.UpdateValues()

	return append([]interface{}{utils.Checksum(n.Id)}, v...)
}

func (n *NotificationUsergroup) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(n.NotificationId),
		utils.Checksum(n.UsergroupId),
		utils.Checksum(n.EnvId),
	)

	return v
}

func (n *NotificationUsergroup) GetId() string {
	return n.Id
}

func (n *NotificationUsergroup) SetId(id string) {
	n.Id = id
}

func (n *NotificationUsergroup) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{n}, nil
}

func init() {
	name := "notification_usergroup"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "notification:usergroup",
		PrimaryMySqlField: "id",
		Factory: NewNotificationUsergroup,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notification",
	}
}