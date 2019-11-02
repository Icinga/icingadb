package checkcommand

import (
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"name",
		"name_ci",
		"zone_id",
		"command",
		"timeout",
	}
)

type CheckCommand struct {
	Id                 string  `json:"id"`
	EnvId              string  `json:"environment_id"`
	NameChecksum       string  `json:"name_checksum"`
	PropertiesChecksum string  `json:"checksum"`
	Name               string  `json:"name"`
	NameCi             *string `json:"name_ci"`
	ZoneId             string  `json:"zone_id"`
	Command            string  `json:"command"`
	Timeout            float32 `json:"timeout"`
}

func NewCheckCommand() connection.Row {
	c := CheckCommand{}
	c.NameCi = &c.Name

	return &c
}

func (c *CheckCommand) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *CheckCommand) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.NameChecksum),
		utils.EncodeChecksum(c.PropertiesChecksum),
		c.Name,
		c.NameCi,
		utils.EncodeChecksum(c.ZoneId),
		c.Command,
		c.Timeout,
	)

	return v
}

func (c *CheckCommand) GetId() string {
	return c.Id
}

func (c *CheckCommand) SetId(id string) {
	c.Id = id
}

func (c *CheckCommand) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "checkcommand"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewCheckCommand,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "checkcommand",
	}
}
