package notesurl

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
		"notes_url",
	}
)

type NotesUrl struct {
	Id               string  `json:"id"`
	EnvId            string  `json:"env_id"`
	NotesUrl       	 string  `json:"notes_url"`
}

func NewNotesUrl() connection.Row {
	a := NotesUrl{}

	return &a
}

func (a *NotesUrl) InsertValues() []interface{} {
	v := a.UpdateValues()

	return append([]interface{}{utils.Checksum(a.Id)}, v...)
}

func (a *NotesUrl) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(a.EnvId),
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

func init() {
	name := "notes_url"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		DeltaMySqlField: "id",
		Factory: NewNotesUrl,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}