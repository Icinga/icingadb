package iconimage

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
		"icon_image",
	}
)

type IconImage struct {
	Id               string  `json:"id"`
	EnvId            string  `json:"env_id"`
	IconImage        string  `json:"icon_image"`
}

func NewIconImage() connection.Row {
	a := IconImage{}

	return &a
}

func (a *IconImage) InsertValues() []interface{} {
	v := a.UpdateValues()

	return append([]interface{}{utils.Checksum(a.Id)}, v...)
}

func (a *IconImage) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(a.EnvId),
		a.IconImage,
	)

	return v
}

func (a *IconImage) GetId() string {
	return a.Id
}

func (a *IconImage) SetId(id string) {
	a.Id = id
}

func init() {
	name := "icon_image"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		DeltaMySqlField: "id",
		Factory: NewIconImage,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}