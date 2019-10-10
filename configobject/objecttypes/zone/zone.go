package zone

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
		"parents_checksum",
		"name",
		"name_ci",
		"is_global",
		"parent_id",
		"depth",
	}
)

type Zone struct {
	Id                  string  `json:"id"`
	EnvId               string  `json:"environment_id"`
	NameChecksum        string  `json:"name_checksum"`
	PropertiesChecksum  string  `json:"checksum"`
	ParentsChecksum     string  `json:"parents_checksum"`
	Name                string  `json:"name"`
	NameCi              *string `json:"name_ci"`
	IsGlobal			bool	`json:"is_global"`
	ParentId            string  `json:"parent_id"`
	Depth				float64	`json:"depth"`
}

func NewZone() connection.Row {
	z := Zone{}
	z.NameCi = &z.Name

	return &z
}

func (z *Zone) InsertValues() []interface{} {
	v := z.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(z.Id)}, v...)
}

func (z *Zone) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(z.EnvId),
		utils.EncodeChecksum(z.NameChecksum),
		utils.EncodeChecksum(z.PropertiesChecksum),
		utils.EncodeChecksum(z.ParentsChecksum),
		z.Name,
		z.NameCi,
		utils.Bool[z.IsGlobal],
		utils.EncodeChecksum(z.ParentId),
		z.Depth,
	)

	return v
}

func (z *Zone) GetId() string {
	return z.Id
}

func (z *Zone) SetId(id string) {
	z.Id = id
}

func (z *Zone) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{z}, nil
}

func init() {
	name := "zone"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		PrimaryMySqlField: "id",
		Factory: NewZone,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "zone",
	}
}