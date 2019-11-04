// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package hostgroupmember

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"hostgroup_id",
		"host_id",
		"environment_id",
	}
)

type HostgroupMember struct {
	Id          string `json:"id"`
	HostgroupId string `json:"group_id"`
	HostId      string `json:"object_id"`
	EnvId       string `json:"environment_id"`
}

func NewHostgroupMember() connection.Row {
	h := HostgroupMember{}
	return &h
}

func (c *HostgroupMember) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (h *HostgroupMember) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(h.HostgroupId),
		utils.EncodeChecksum(h.HostId),
		utils.EncodeChecksum(h.EnvId),
	)

	return v
}

func (h *HostgroupMember) GetId() string {
	return h.Id
}

func (h *HostgroupMember) SetId(id string) {
	h.Id = id
}

func (h *HostgroupMember) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "hostgroup_member"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "host:groupmember",
		PrimaryMySqlField:        "id",
		Factory:                  NewHostgroupMember,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "host",
	}
}
