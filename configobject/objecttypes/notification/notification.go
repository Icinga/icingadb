// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package notification

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"customvars_checksum",
		"users_checksum",
		"usergroups_checksum",
		"name",
		"name_ci",
		"host_id",
		"service_id",
		"command_id",
		"times_begin",
		"times_end",
		"notification_interval",
		"timeperiod_id",
		"states",
		"types",
		"zone_id",
	}
)

type Notification struct {
	Id                   string   `json:"id"`
	EnvId                string   `json:"environment_id"`
	NameChecksum         string   `json:"name_checksum"`
	PropertiesChecksum   string   `json:"checksum"`
	CustomvarsChecksum   string   `json:"customvars_checksum"`
	UsersChecksum        string   `json:"users_checksum"`
	UsergroupsChecksum   string   `json:"usergroups_checksum"`
	Name                 string   `json:"name"`
	NameCi               *string  `json:"name_ci"`
	HostId               string   `json:"host_id"`
	ServiceId            string   `json:"service_id"`
	CommandId            string   `json:"command_id"`
	TimesBegin           float32  `json:"times_begin"`
	TimesEnd             float32  `json:"times_end"`
	NotificationInterval float32  `json:"notification_interval"`
	PeriodId             string   `json:"timeperiod_id"`
	States               []string `json:"states"`
	Types                []string `json:"types"`
	ZoneId               string   `json:"zone_id"`
}

func NewNotification() connection.Row {
	n := Notification{}
	n.NameCi = &n.Name

	return &n
}

func (n *Notification) InsertValues() []interface{} {
	v := n.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(n.Id)}, v...)
}

func (n *Notification) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(n.EnvId),
		utils.EncodeChecksum(n.NameChecksum),
		utils.EncodeChecksum(n.PropertiesChecksum),
		utils.EncodeChecksum(n.CustomvarsChecksum),
		utils.EncodeChecksum(n.UsersChecksum),
		utils.EncodeChecksum(n.UsergroupsChecksum),
		n.Name,
		n.NameCi,
		utils.EncodeChecksum(n.HostId),
		utils.EncodeChecksum(n.ServiceId),
		utils.EncodeChecksum(n.CommandId),
		n.TimesBegin,
		n.TimesEnd,
		n.NotificationInterval,
		utils.EncodeChecksum(n.PeriodId),
		utils.NotificationStatesToBitMask(n.States),
		utils.NotificationTypesToBitMask(n.Types),
		utils.EncodeChecksum(n.ZoneId),
	)

	return v
}

func (n *Notification) GetId() string {
	return n.Id
}

func (n *Notification) SetId(id string) {
	n.Id = id
}

func (n *Notification) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{n}, nil
}

func init() {
	name := "notification"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewNotification,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "notification",
	}
}
