package hostcustomvar

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"host_id",
		"customvar_id",
		"environment_id",
	}
)

type HostCustomvar struct {
	Id          string `json:"id"`
	HostId      string `json:"object_id"`
	CustomvarId string `json:"customvar_id"`
	EnvId       string `json:"environment_id"`
}

func NewHostCustomvar() connection.Row {
	c := HostCustomvar{}
	return &c
}

func (c *HostCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *HostCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.HostId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *HostCustomvar) GetId() string {
	return c.Id
}

func (c *HostCustomvar) SetId(id string) {
	c.Id = id
}

func (h *HostCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "host_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "host:customvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewHostCustomvar,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "host",
	}
}
