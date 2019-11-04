package comment

import (
	"fmt"
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"object_type",
		"host_id",
		"service_id",
		"name_checksum",
		"properties_checksum",
		"name",
		"author",
		"text",
		"entry_type",
		"entry_time",
		"is_persistent",
		"is_sticky",
		"expire_time",
		"zone_id",
	}
)

type Comment struct {
	Id                 string  `json:"id"`
	EnvId              string  `json:"environment_id"`
	ObjectType         string  `json:"object_type"`
	HostId             string  `json:"host_id"`
	ServiceId          string  `json:"service_id"`
	NameChecksum       string  `json:"name_checksum"`
	PropertiesChecksum string  `json:"checksum"`
	Name               string  `json:"name"`
	Author             string  `json:"author"`
	Text               string  `json:"text"`
	EntryType          float64 `json:"entry_type"`
	EntryTime          float64 `json:"entry_time"`
	IsPersistent       bool    `json:"is_persistent"`
	IsSticky           bool    `json:"is_sticky"`
	ExpireTime         float64 `json:"expire_time"`
	ZoneId             string  `json:"zone_id"`
}

func NewComment() connection.Row {
	c := Comment{}

	return &c
}

func (c *Comment) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *Comment) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.EnvId),
		c.ObjectType,
		utils.EncodeChecksum(c.HostId),
		utils.EncodeChecksum(c.ServiceId),
		utils.EncodeChecksum(c.NameChecksum),
		utils.EncodeChecksum(c.PropertiesChecksum),
		c.Name,
		c.Author,
		c.Text,
		utils.CommentEntryTypes[fmt.Sprintf("%.0f", c.EntryType)],
		c.EntryTime,
		utils.Bool[c.IsPersistent],
		utils.Bool[c.IsSticky],
		c.ExpireTime,
		utils.EncodeChecksum(c.ZoneId),
	)

	return v
}

func (c *Comment) GetId() string {
	return c.Id
}

func (c *Comment) SetId(id string) {
	c.Id = id
}

func (c *Comment) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "comment"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "comment",
		PrimaryMySqlField:        "id",
		Factory:                  NewComment,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "comment",
	}
}
