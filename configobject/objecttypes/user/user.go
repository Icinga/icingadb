package user

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"customvars_checksum",
		"groups_checksum",
		"name",
		"name_ci",
		"display_name",
		"email",
		"pager",
		"notifications_enabled",
		"timeperiod_id",
		"states",
		"types",
		"zone_id",
	}
)

type User struct {
	Id						string  	`json:"id"`
	EnvId					string  	`json:"environment_id"`
	NameChecksum			string  	`json:"name_checksum"`
	PropertiesChecksum  	string  	`json:"checksum"`
	CustomvarsChecksum  	string  	`json:"customvars_checksum"`
	GroupsChecksum      	string  	`json:"groups_checksum"`
	Name                	string  	`json:"name"`
	NameCi              	*string 	`json:"name_ci"`
	DisplayName         	string  	`json:"display_name"`
	EMail           		string  	`json:"email"`
	Pager           		string  	`json:"pager"`
	NotificationsEnabled	bool 		`json:"notifications_enabled"`
	PeriodId				string 		`json:"timeperiod_id"`
	States           		[]string  	`json:"states"`
	Types           		[]string	`json:"types"`
	ZoneId              	string  	`json:"zone_id"`
}

func NewUser() connection.Row {
	u := User{}
	u.NameCi = &u.Name

	return &u
}

func (u *User) InsertValues() []interface{} {
	v := u.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(u.Id)}, v...)
}

func (u *User) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(u.EnvId),
		utils.EncodeChecksum(u.NameChecksum),
		utils.EncodeChecksum(u.PropertiesChecksum),
		utils.EncodeChecksum(u.CustomvarsChecksum),
		utils.EncodeChecksum(u.GroupsChecksum),
		u.Name,
		u.NameCi,
		u.DisplayName,
		u.EMail,
		u.Pager,
		u.NotificationsEnabled,
		utils.EncodeChecksum(u.PeriodId),
		utils.NotificationStatesToBitMask(u.States),
		utils.NotificationTypesToBitMask(u.Types),
		utils.EncodeChecksum(u.ZoneId),
	)

	return v
}

func (u *User) GetId() string {
	return u.Id
}

func (u *User) SetId(id string) {
	u.Id = id
}

func (u *User) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{u}, nil
}

func init() {
	name := "user"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		PrimaryMySqlField: "id",
		Factory: NewUser,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "user",
	}
}