package actionurl

import (
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"action_url",
	}
)

type ActionUrl struct {
	Id        string `json:"id"`
	EnvId     string `json:"environment_id"`
	ActionUrl string `json:"action_url"`
}

func NewActionUrl() connection.Row {
	a := ActionUrl{}

	return &a
}

func (a *ActionUrl) InsertValues() []interface{} {
	v := a.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(a.Id)}, v...)
}

func (a *ActionUrl) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(a.EnvId),
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
		ObjectType:        name,
		RedisKey:          name,
		PrimaryMySqlField: "id",
		Factory:           NewActionUrl,
		HasChecksum:       false,
		BulkInsertStmt:    connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:    connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:    connection.NewBulkUpdateStmt(name, Fields),
	}
}
