// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package customvar

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
		"name",
		"value",
	}
)

type Customvar struct {
	Id           string `json:"id"`
	EnvId        string `json:"environment_id"`
	NameChecksum string `json:"name_checksum"`
	Name         string `json:"name"`
	Value        string `json:"value"`
}

func NewCustomvar() connection.Row {
	c := Customvar{}

	return &c
}

func (c *Customvar) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *Customvar) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	val, truncated := utils.TruncText(c.Value, 65535)
	if truncated {
		log.WithFields(log.Fields{
			"Table": "customvar",
			"Column": "value",
			"id": c.Id,
		}).Infof("Truncated custom variable value to 64KB")
	}

	v = append(
		v,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.NameChecksum),
		c.Name,
		val,
	)

	return v
}

func (c *Customvar) GetId() string {
	return c.Id
}

func (c *Customvar) SetId(id string) {
	c.Id = id
}

func (c *Customvar) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "customvar"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:        name,
		RedisKey:          name,
		PrimaryMySqlField: "id",
		Factory:           NewCustomvar,
		HasChecksum:       false,
		BulkInsertStmt:    connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:    connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:    connection.NewBulkUpdateStmt(name, Fields),
	}
}
