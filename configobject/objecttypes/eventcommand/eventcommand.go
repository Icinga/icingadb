package eventcommand

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"env_id",
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
	Id                    string  	`json:"id"`
	EnvId                 string  	`json:"env_id"`
	NameChecksum          string  	`json:"name_checksum"`
	PropertiesChecksum    string  	`json:"checksum"`
	Name                  string  	`json:"name"`
	NameCi                *string 	`json:"name_ci"`
	ZoneId                string  	`json:"zone_id"`
	Command               string  	`json:"command"`
	Timeout               float32	`json:"timeout"`
}

func NewEventCommand() connection.Row {
	c := EventCommand{}
	c.NameCi = &c.Name

	return &c
}

func (c *EventCommand) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.Checksum(c.Id)}, v...)
}

func (c *EventCommand) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(c.EnvId),
		utils.Checksum(c.NameChecksum),
		utils.Checksum(c.PropertiesChecksum),
		c.Name,
		c.NameCi,
		utils.Checksum(c.ZoneId),
		c.Command,
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

func init() {
	name := "eventcommand"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: name,
		DeltaMySqlField: "id",
		Factory: NewEventCommand,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
	}
}