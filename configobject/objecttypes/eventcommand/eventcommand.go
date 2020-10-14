// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package eventcommand

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

type EventCommand struct {
	Id                 string  `json:"id"`
	EnvId              string  `json:"environment_id"`
	NameChecksum       string  `json:"name_checksum"`
	PropertiesChecksum string  `json:"checksum"`
	Name               string  `json:"name"`
	NameCi             *string `json:"name_ci"`
	ZoneId             string  `json:"zone_id"`
	Command            string  `json:"command"`
	Timeout            float64 `json:"timeout"`
}

func NewEventCommand() connection.Row {
	c := EventCommand{}
	c.NameCi = &c.Name

	return &c
}

func (c *EventCommand) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *EventCommand) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	cmd, truncated := utils.TruncText(c.Command, 65535)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "eventcommand",
			"Column": "command",
			"id": c.Id,
		}).Infof("Truncated event command to 64KB")
	}

	v = append(
		v,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.NameChecksum),
		utils.EncodeChecksum(c.PropertiesChecksum),
		c.Name,
		c.NameCi,
		utils.EncodeChecksumOrNil(c.ZoneId),
		cmd,
		c.Timeout,
	)

	return v
}

func (c *EventCommand) GetId() string {
	return c.Id
}

func (c *EventCommand) SetId(id string) {
	c.Id = id
}

func (c *EventCommand) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "eventcommand"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewEventCommand,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "eventcommand",
	}
}
