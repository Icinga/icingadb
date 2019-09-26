package endpoint

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
		"name",
		"name_ci",
		"zone_id",
	}
)

type Endpoint struct {
	Id                  string  `json:"id"`
	EnvId               string  `json:"environment_id"`
	NameChecksum        string  `json:"name_checksum"`
	PropertiesChecksum  string  `json:"checksum"`
	Name                string  `json:"name"`
	NameCi              *string `json:"name_ci"`
	ZoneId            	string  `json:"zone_id"`
}

func NewEndpoint() connection.Row {
	e := Endpoint{}
	e.NameCi = &e.Name

	return &e
}

func (e *Endpoint) InsertValues() []interface{} {
	v := e.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(e.Id)}, v...)
}

func (e *Endpoint) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(e.EnvId),
		utils.EncodeChecksum(e.NameChecksum),
		utils.EncodeChecksum(e.PropertiesChecksum),
		e.Name,
		e.NameCi,
		utils.EncodeChecksum(e.ZoneId),
	)

	return v
}

func (e *Endpoint) GetId() string {
	return e.Id
}

func (e *Endpoint) SetId(id string) {
	e.Id = id
}

func (e *Endpoint) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{e}, nil
}

func init() {
	name := "endpoint"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		PrimaryMySqlField: "id",
		Factory: NewEndpoint,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "endpoint",
	}
}