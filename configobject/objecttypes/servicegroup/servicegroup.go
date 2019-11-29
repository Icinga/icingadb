// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package servicegroup

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"name",
		"name_ci",
		"display_name",
		"zone_id",
	}
)

type Servicegroup struct {
	Id                 string  `json:"id"`
	EnvId              string  `json:"environment_id"`
	NameChecksum       string  `json:"name_checksum"`
	PropertiesChecksum string  `json:"checksum"`
	Name               string  `json:"name"`
	NameCi             *string `json:"name_ci"`
	DisplayName        string  `json:"display_name"`
	ZoneId             string  `json:"zone_id"`
}

func NewServicegroup() connection.Row {
	s := Servicegroup{}
	s.NameCi = &s.Name

	return &s
}

func (s *Servicegroup) InsertValues() []interface{} {
	v := s.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(s.Id)}, v...)
}

func (s *Servicegroup) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(s.EnvId),
		utils.EncodeChecksum(s.NameChecksum),
		utils.EncodeChecksum(s.PropertiesChecksum),
		s.Name,
		s.NameCi,
		s.DisplayName,
		utils.EncodeChecksum(s.ZoneId),
	)

	return v
}

func (s *Servicegroup) GetId() string {
	return s.Id
}

func (s *Servicegroup) SetId(id string) {
	s.Id = id
}

func (s *Servicegroup) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{s}, nil
}

func init() {
	name := "servicegroup"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewServicegroup,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "servicegroup",
	}
}
