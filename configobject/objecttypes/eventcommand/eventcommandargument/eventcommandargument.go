package eventcommandargument

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

type EventCommandArgument struct {
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

func NewEventCommandArgument() connection.Row {
	c := EventCommandArgument{}
	return &c
}

func (c *EventCommandArgument) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *EventCommandArgument) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.CommandId),
		c.ArgumentKey,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.PropertiesChecksum),
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

func (c *EventCommandArgument) GetId() string {
	return c.Id
}

func (c *EventCommandArgument) SetId(id string) {
	c.Id = id
}

func (c *EventCommandArgument) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "eventcommand_argument"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "eventcommand:argument",
		PrimaryMySqlField: "id",
		Factory: NewEventCommandArgument,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "eventcommand",
	}
}