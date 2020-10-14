// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package checkcommandargument

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
	log "github.com/sirupsen/logrus"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
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

type CheckCommandArgument struct {
	Id                  string      `json:"id"`
	CommandId           string      `json:"command_id"`
	ArgumentKey         string      `json:"argument_key"`
	EnvId               string      `json:"environment_id"`
	PropertiesChecksum  string      `json:"checksum"`
	ArgumentValue       string      `json:"value"`
	ArgumentOrder       float32     `json:"order"`
	Description         string      `json:"description"`
	ArgumentKeyOverride string      `json:"key"`
	RepeatKey           bool        `json:"repeat_key"`
	Required            bool        `json:"required"`
	SetIf               interface{} `json:"set_if"`
	SkipKey             bool        `json:"skip_key"`
}

func NewCheckCommandArgument() connection.Row {
	c := CheckCommandArgument{}
	return &c
}

func (c *CheckCommandArgument) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *CheckCommandArgument) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	argVal, truncated := utils.TruncText(c.ArgumentValue, 65535)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "checkcommand_argument",
			"Column": "argument_value",
			"id": c.Id,
		}).Infof("Truncated check command argument value to 64KB")
	}

	desc, truncated := utils.TruncText(c.Description, 65535)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "checkcommand_argument",
			"Column": "description",
			"id": c.Id,
		}).Infof("Truncated check command description to 64KB")
	}

	v = append(
		v,
		utils.EncodeChecksum(c.CommandId),
		c.ArgumentKey,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.PropertiesChecksum),
		argVal,
		c.ArgumentOrder,
		desc,
		c.ArgumentKeyOverride,
		utils.Bool[c.RepeatKey],
		utils.Bool[c.Required],
		connection.ConvertValueForDb(c.SetIf),
		utils.Bool[c.SkipKey],
	)

	return v
}

func (c *CheckCommandArgument) GetId() string {
	return c.Id
}

func (c *CheckCommandArgument) SetId(id string) {
	c.Id = id
}

func (c *CheckCommandArgument) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "checkcommand_argument"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "checkcommand:argument",
		PrimaryMySqlField:        "id",
		Factory:                  NewCheckCommandArgument,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "checkcommand",
	}
}
