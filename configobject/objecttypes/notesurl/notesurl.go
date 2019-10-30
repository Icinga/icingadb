package notesurl

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"notes_url",
	}
)

type NotesUrl struct {
	Id       string `json:"id"`
	EnvId    string `json:"environment_id"`
	NotesUrl string `json:"notes_url"`
}

func NewNotesUrl() connection.Row {
	a := NotesUrl{}

	return &a
}

func (a *NotesUrl) InsertValues() []interface{} {
	v := a.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(a.Id)}, v...)
}

func (a *NotesUrl) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(a.EnvId),
		a.NotesUrl,
	)

	return v
}

func (a *NotesUrl) GetId() string {
	return a.Id
}

func (a *NotesUrl) SetId(id string) {
	a.Id = id
}

func (a *NotesUrl) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{a}, nil
}

func init() {
	name := "notes_url"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:        name,
		RedisKey:          name,
		PrimaryMySqlField: "id",
		Factory:           NewNotesUrl,
		HasChecksum:       false,
		BulkInsertStmt:    connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:    connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:    connection.NewBulkUpdateStmt(name, Fields),
	}
}
