package actionurl

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields = []string{
		"id",
		"env_id",
		"action_url",
	}
)

type ActionUrl struct {
	Id               string  `json:"id"`
	EnvId            string  `json:"env_id"`
	ActionUrl        string  `json:"action_url"`
}

func NewActionUrl() connection.Row {
	a := ActionUrl{}

	return &a
}

func (a *ActionUrl) InsertValues() []interface{} {
	v := a.UpdateValues()

	return append([]interface{}{utils.Checksum(a.Id)}, v...)
}

func (a *ActionUrl) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(a.EnvId),
		a.ActionUrl,
	)

	return v
}

func (a *ActionUrl) GetId() string {
	return a.Id
}

func (a *ActionUrl) SetId(id string) {
	a.Id = id
}

func (a *ActionUrl) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{a}, nil
}

func init() {
	name := "action_url"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		PrimaryMySqlField: "id",
		Factory: NewActionUrl,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}