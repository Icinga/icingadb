package hostgroupcustomvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"hostgroup_id",
		"customvar_id",
		"environment_id",
	}
)

type HostgroupCustomvar struct {
	Id						string 		`json:"id"`
	HostgroupId				string		`json:"object_id"`
	CustomvarId 			string 		`json:"customvar_id"`
	EnvId           		string		`json:"environment_id"`
}

func NewHostgroupCustomvar() connection.Row {
	c := HostgroupCustomvar{}
	return &c
}

func (c *HostgroupCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *HostgroupCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.HostgroupId),
		utils.Checksum(c.CustomvarId),
		utils.Checksum(c.EnvId),
	)

	return v
}

func (c *HostgroupCustomvar) GetId() string {
	return c.Id
}

func (c *HostgroupCustomvar) SetId(id string) {
	c.Id = id
}

func (h *HostgroupCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "hostgroup_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "hostgroup:customvar",
		PrimaryMySqlField: "id",
		Factory: NewHostgroupCustomvar,
		HasChecksum: false,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "hostgroup",
	}
}