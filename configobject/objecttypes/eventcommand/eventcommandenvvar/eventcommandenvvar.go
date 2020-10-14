// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package eventcommandenvvar

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
		"envvar_key",
		"environment_id",
		"properties_checksum",
		"envvar_value",
	}
)

type EventCommandEnvvar struct {
	Id                 string `json:"id"`
	CommandId          string `json:"command_id"`
	EnvvarKey          string `json:"envvar_key"`
	EnvId              string `json:"environment_id"`
	PropertiesChecksum string `json:"checksum"`
	EnvvarValue        string `json:"value"`
}

func NewEventCommandEnvvar() connection.Row {
	c := EventCommandEnvvar{}
	return &c
}

func (c *EventCommandEnvvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *EventCommandEnvvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	envvarVal, truncated := utils.TruncText(c.EnvvarValue, 65536)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "eventcommand_envvar",
			"Column": "envvar_value",
			"id": c.Id,
		}).Infof("Truncated event command environment variable value to 64KB")
	}

	v = append(
		v,
		utils.EncodeChecksum(c.CommandId),
		c.EnvvarKey,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.PropertiesChecksum),
		envvarVal,
	)

	return v
}

func (c *EventCommandEnvvar) GetId() string {
	return c.Id
}

func (c *EventCommandEnvvar) SetId(id string) {
	c.Id = id
}

func (c *EventCommandEnvvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "eventcommand_envvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "eventcommand:envvar",
		PrimaryMySqlField:        "id",
		Factory:                  NewEventCommandEnvvar,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "eventcommand",
	}
}
