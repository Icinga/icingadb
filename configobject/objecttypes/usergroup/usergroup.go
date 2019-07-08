package usergroup

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"env_id",
		"name_checksum",
		"properties_checksum",
		"customvars_checksum",
		"name",
		"name_ci",
		"display_name",
		"zone_id",
	}
)

type Usergroup struct {
	Id                    string  `json:"id"`
	EnvId                 string  `json:"environment_id"`
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"properties_checksum"`
	CustomvarsChecksum    string  `json:"customvars_checksum"`
	Name                  string  `json:"name"`
	NameCi                *string `json:"name_ci"`
	DisplayName           string  `json:"display_name"`
	ZoneId                string  `json:"zone_id"`
}

func NewUsergroup() connection.Row {
	u := Usergroup{}
	u.NameCi = &u.Name

	return &u
}

func (u *Usergroup) InsertValues() []interface{} {
	v := u.UpdateValues()

	return append([]interface{}{utils.Checksum(u.Id)}, v...)
}

func (u *Usergroup) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(u.EnvId),
		utils.Checksum(u.NameChecksum),
		utils.Checksum(u.PropertiesChecksum),
		utils.Checksum(u.CustomvarsChecksum),
		u.Name,
		u.NameCi,
		u.DisplayName,
		utils.Checksum(u.ZoneId),
	)

	return v
}

func (u *Usergroup) GetId() string {
	return u.Id
}

func (u *Usergroup) SetId(id string) {
	u.Id = id
}

func init() {
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: "usergroup",
		RedisKey: "usergroup",
		Factory: NewUsergroup,
		BulkInsertStmt: connection.NewBulkInsertStmt("usergroup", Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt("usergroup"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt("usergroup", Fields),
	}
}