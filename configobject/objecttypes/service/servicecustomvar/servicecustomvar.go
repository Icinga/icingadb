package servicecustomvar

import (
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"service_id",
		"customvar_id",
		"environment_id",
	}
)

type ServiceCustomvar struct {
	Id          string `json:"id"`
	ServiceId   string `json:"object_id"`
	CustomvarId string `json:"customvar_id"`
	EnvId       string `json:"environment_id"`
}

func NewServiceCustomvar() connection.Row {
	c := ServiceCustomvar{}
	return &c
}

func (c *ServiceCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *ServiceCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.ServiceId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *ServiceCustomvar) GetId() string {
	return c.Id
}

func (c *ServiceCustomvar) SetId(id string) {
	c.Id = id
}

func (c *ServiceCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "service_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "service:customvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewServiceCustomvar,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "service",
	}
}
