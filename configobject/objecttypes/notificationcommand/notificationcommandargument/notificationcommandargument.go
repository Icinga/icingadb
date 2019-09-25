package notificationcommandargument

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
		"argument_key",
		"environment_id",
		"properties_checksum",
		"argument_value",
		"argument_order",
		"description",
		"argument_key_override",
		"repeat_key",
		"required",
		"set_if",
		"skip_key",
	}
)

type NotificationCommandArgument struct {
	Id						string 		`json:"id"`
	CommandId				string		`json:"command_id"`
	ArgumentKey				string		`json:"argument_key"`
	EnvId           		string		`json:"environment_id"`
	PropertiesChecksum		string		`json:"checksum"`
	ArgumentValue			string		`json:"value"`
	ArgumentOrder			float32		`json:"order"`
	Description				string		`json:"description"`
	ArgumentKeyOverride		string		`json:"key"`
	RepeatKey				bool		`json:"repeat_key"`
	Required				bool		`json:"required"`
	SetIf					string		`json:"set_if"`
	SkipKey					bool		`json:"skip_key"`
}

func NewNotificationCommandArgument() connection.Row {
	c := NotificationCommandArgument{}
	return &c
}

func (c *NotificationCommandArgument) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *NotificationCommandArgument) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.CommandId),
		c.ArgumentKey,
		utils.Checksum(c.EnvId),
		utils.Checksum(c.PropertiesChecksum),
		c.ArgumentValue,
		c.ArgumentOrder,
		c.Description,
		c.ArgumentKeyOverride,
		utils.Bool[c.RepeatKey],
		utils.Bool[c.Required],
		c.SetIf,
		utils.Bool[c.SkipKey],
	)

	return v
}

func (c *NotificationCommandArgument) GetId() string {
	return c.Id
}

func (c *NotificationCommandArgument) SetId(id string) {
	c.Id = id
}

func (c *NotificationCommandArgument) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "notificationcommand_argument"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "notificationcommand:argument",
		PrimaryMySqlField: "id",
		Factory: NewNotificationCommandArgument,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notificationcommand",
	}
}