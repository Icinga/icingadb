package servicegroup

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

type Servicegroup struct {
	Id                    string  `json:"id"`
	EnvId                 string  `json:"env_id"`
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"checksum"`
	CustomvarsChecksum    string  `json:"customvars_checksum"`
	Name                  string  `json:"name"`
	NameCi                *string `json:"name_ci"`
	DisplayName           string  `json:"display_name"`
	ZoneId                string  `json:"zone_id"`
}

func NewServicegroup() connection.Row {
	s := Servicegroup{}
	s.NameCi = &s.Name

	return &s
}

func (s *Servicegroup) InsertValues() []interface{} {
	v := s.UpdateValues()

	return append([]interface{}{utils.Checksum(s.Id)}, v...)
}

func (s *Servicegroup) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(s.EnvId),
		utils.Checksum(s.NameChecksum),
		utils.Checksum(s.PropertiesChecksum),
		utils.Checksum(s.CustomvarsChecksum),
		s.Name,
		s.NameCi,
		s.DisplayName,
		utils.Checksum(s.ZoneId),
	)

	return v
}

func (s *Servicegroup) GetId() string {
	return s.Id
}

func (s *Servicegroup) SetId(id string) {
	s.Id = id
}

func init() {
	name := "servicegroup"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		DeltaMySqlField: "id",
		Factory: NewServicegroup,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}