package hostcustomvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"host_id",
		"customvar_id",
		"env_id",
	}
)

type HostCustomvar struct {
	Id						string 		`json:"id"`
	HostId					string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"env_id"`
}

func NewHostCustomvar() connection.Row {
	c := HostCustomvar{}
	return &c
}

func (c *HostCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.ChecksumString{c.Id}}, v...)
}

func (c *HostCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.ChecksumString{c.HostId},
		utils.ChecksumString{c.CustomvarId},
		utils.ChecksumString{c.EnvId},
	)

	return v
}

func (c *HostCustomvar) GetId() string {
	return c.Id
}

func (c *HostCustomvar) SetId(id string) {
	c.Id = id
}

func init() {
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: "host_customvar",
		RedisKey: "host:customvar",
		Factory: NewHostCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt("host_customvar", Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt("host_customvar"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt("host_customvar", Fields),
	}
}