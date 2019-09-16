package checkcommandenvvar

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"command_id",
		"envvar_key",
		"env_id",
		"properties_checksum",
		"envvar_value",
	}
)

type CheckCommandEnvvar struct {
	Id						string 		`json:"id"`
	CommandId				string		`json:"command_id"`
	EnvvarKey				string		`json:"envvar_key"`
	EnvId           		string		`json:"env_id"`
	PropertiesChecksum		string		`json:"checksum"`
	EnvvarValue				string		`json:"value"`
}

func NewCheckCommandEnvvar() connection.Row {
	c := CheckCommandEnvvar{}
	return &c
}

func (c *CheckCommandEnvvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *CheckCommandEnvvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.CommandId),
		c.EnvvarKey,
		utils.Checksum(c.EnvId),
		utils.Checksum(c.PropertiesChecksum),
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
		ObjectType: name,
		RedisKey: "checkcommand:envvar",
		DeltaMySqlField: "id",
		Factory: NewCheckCommandEnvvar,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "checkcommand",
	}
}