// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package notificationcommand

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

type NotificationCommand struct {
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

func NewNotificationCommand() connection.Row {
	c := NotificationCommand{}
	c.NameCi = &c.Name

	return &c
}

func (c *NotificationCommand) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *NotificationCommand) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	cmd, truncated := utils.TruncText(c.Command, 65535)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "notificationcommand",
			"Column": "command",
			"id": c.Id,
		}).Infof("Truncated notification command to 64KB")
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

func (c *NotificationCommand) GetId() string {
	return c.Id
}

func (c *NotificationCommand) SetId(id string) {
	c.Id = id
}

func (c *NotificationCommand) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "notificationcommand"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewNotificationCommand,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notificationcommand",
	}
}
