// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package checkcommandenvvar

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
		"envvar_key",
		"environment_id",
		"properties_checksum",
		"envvar_value",
	}
)

type CheckCommandEnvvar struct {
	Id                 string `json:"id"`
	CommandId          string `json:"command_id"`
	EnvvarKey          string `json:"envvar_key"`
	EnvId              string `json:"environment_id"`
	PropertiesChecksum string `json:"checksum"`
	EnvvarValue        string `json:"value"`
}

func NewCheckCommandEnvvar() connection.Row {
	c := CheckCommandEnvvar{}
	return &c
}

func (c *CheckCommandEnvvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *CheckCommandEnvvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.CommandId),
		c.EnvvarKey,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.PropertiesChecksum),
		c.EnvvarValue,
	)

	return v
}

func (c *CheckCommandEnvvar) GetId() string {
	return c.Id
}

func (c *CheckCommandEnvvar) SetId(id string) {
	c.Id = id
}

func (c *CheckCommandEnvvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "checkcommand_envvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "checkcommand:envvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewCheckCommandEnvvar,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "checkcommand",
	}
}
