package usergroupmember

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"usergroup_id",
		"user_id",
		"env_id",
	}
)

type UsergroupMember struct {
	Id					string 		`json:"id"`
	UsergroupId			string		`json:"group_id"`
	UserId	 			string 		`json:"user_id"`
	EnvId           	string		`json:"env_id"`
}

func NewUsergroupMember() connection.Row {
	u := UsergroupMember{}
	return &u
}

func (u *UsergroupMember) InsertValues() []interface{} {
	v := u.UpdateValues()

	return append([]interface{}{utils.Checksum(u.Id)}, v...)
}

func (u *UsergroupMember) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(u.UsergroupId),
		utils.Checksum(u.UserId),
		utils.Checksum(u.EnvId),
	)

	return v
}

func (u *UsergroupMember) GetId() string {
	return u.Id
}

func (u *UsergroupMember) SetId(id string) {
	u.Id = id
}

func (u *UsergroupMember) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{u}, nil
}

func init() {
	name := "usergroup_member"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "user:groupmember",
		DeltaMySqlField: "id",
		Factory: NewUsergroupMember,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "user",
	}
}