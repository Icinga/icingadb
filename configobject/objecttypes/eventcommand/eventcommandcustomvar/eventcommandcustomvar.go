package eventcommandcustomvar

import (
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"command_id",
		"customvar_id",
		"environment_id",
	}
)

type EventCommandCustomvar struct {
	Id             string `json:"id"`
	EventCommandId string `json:"object_id"`
	CustomvarId    string `json:"customvar_id"`
	EnvId          string `json:"environment_id"`
}

func NewEventCommandCustomvar() connection.Row {
	c := EventCommandCustomvar{}
	return &c
}

func (c *EventCommandCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *EventCommandCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.EventCommandId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *EventCommandCustomvar) GetId() string {
	return c.Id
}

func (c *EventCommandCustomvar) SetId(id string) {
	c.Id = id
}

func (c *EventCommandCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "eventcommand_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "eventcommand:customvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewEventCommandCustomvar,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "eventcommand",
	}
}
