package hostcomment

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"environment_id",
		"host_id",
		"name_checksum",
		"properties_checksum",
		"name",
		"author",
		"text",
		"entry_type",
		"entry_time",
		"is_persistent",
		"expire_time",
		"zone_id",
	}
)

type HostComment struct {
	Id					string	`json:"id"`
	EnvId               string	`json:"environment_id"`
	HostId           	string	`json:"host_id"`
	NameChecksum        string	`json:"name_checksum"`
	PropertiesChecksum  string	`json:"checksum"`
	Name                string	`json:"name"`
	Author              string	`json:"author"`
	Text               	string	`json:"text"`
	EntryType           float64	`json:"entry_type"`
	EntryTime           float64	`json:"entry_time"`
	IsPersistent      	bool	`json:"is_persistent"`
	ExpireTime          float64	`json:"expire_time"`
	ZoneId              string	`json:"zone_id"`
}

func NewHostComment() connection.Row {
	h := HostComment{}

	return &h
}

func (h *HostComment) InsertValues() []interface{} {
	v := h.UpdateValues()

	return append([]interface{}{utils.Checksum(h.Id)}, v...)
}

func (h *HostComment) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(h.EnvId),
		utils.Checksum(h.HostId),
		utils.Checksum(h.NameChecksum),
		utils.Checksum(h.PropertiesChecksum),
		h.Name,
		h.Author,
		h.Text,
		h.EntryType,
		h.EntryTime,
		utils.Bool[h.IsPersistent],
		h.ExpireTime,
		utils.Checksum(h.ZoneId),
	)

	return v
}

func (h *HostComment) GetId() string {
	return h.Id
}

func (h *HostComment) SetId(id string) {
	h.Id = id
}

func (h *HostComment) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "host_comment"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: name,
		RedisKey: "hostcomment",
		PrimaryMySqlField: "id",
		Factory: NewHostComment,
		HasChecksum: true,
		BulkInsertStmt: connection.NewBulkInsertStmt(name, Fields, "id"),
		BulkDeleteStmt: connection.NewBulkDeleteStmt(name,  "id"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "hostcomment",
	}
}