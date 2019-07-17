package servicegroupcustomvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"servicegroup_id",
		"customvar_id",
		"env_id",
	}
)

type ServicegroupCustomvar struct {
	Id						string 		`json:"id"`
	ServicegroupId			string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"env_id"`
}

func NewServicegroupCustomvar() connection.Row {
	c := ServicegroupCustomvar{}
	return &c
}

func (c *ServicegroupCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *ServicegroupCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.ServicegroupId),
		utils.Checksum(c.CustomvarId),
		utils.Checksum(c.EnvId),
	)

	return v
}

func (c *ServicegroupCustomvar) GetId() string {
	return c.Id
}

func (c *ServicegroupCustomvar) SetId(id string) {
	c.Id = id
}

func init() {
	name := "servicegroup_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "servicegroup:customvar",
		Factory: NewServicegroupCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}