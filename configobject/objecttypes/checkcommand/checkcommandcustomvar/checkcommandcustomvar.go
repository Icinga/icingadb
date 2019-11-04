// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package checkcommandcustomvar

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
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

type CheckCommandCustomvar struct {
	Id             string `json:"id"`
	CheckCommandId string `json:"object_id"`
	CustomvarId    string `json:"customvar_id"`
	EnvId          string `json:"environment_id"`
}

func NewCheckCommandCustomvar() connection.Row {
	c := CheckCommandCustomvar{}
	return &c
}

func (c *CheckCommandCustomvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *CheckCommandCustomvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.CheckCommandId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.EnvId),
	)

	return v
}

func (c *CheckCommandCustomvar) GetId() string {
	return c.Id
}

func (c *CheckCommandCustomvar) SetId(id string) {
	c.Id = id
}

func (c *CheckCommandCustomvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "checkcommand_customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "checkcommand:customvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewCheckCommandCustomvar,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "checkcommand",
	}
}
