package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Notification struct {
	EntityWithChecksum   `json:",inline"`
	EnvironmentMeta      `json:",inline"`
	NameCiMeta           `json:",inline"`
	HostId               types.Binary            `json:"host_id"`
	ServiceId            types.Binary            `json:"service_id"`
	CommandId            types.Binary            `json:"command_id"`
	TimesBegin           types.Int               `json:"times_begin"`
	TimesEnd             types.Int               `json:"times_end"`
	NotificationInterval uint32                  `json:"notification_interval"`
	TimeperiodId         types.Binary            `json:"timeperiod_id"`
	States               uint8                   `json:"states"`
	Types                types.NotificationTypes `json:"types"`
	ZoneId               types.Binary            `json:"zone_id"`
}

type NotificationUser struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NotificationId        types.Binary `json:"notification_id"`
	UserId                types.Binary `json:"user_id"`
}

type NotificationUsergroup struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NotificationId        types.Binary `json:"notification_id"`
	UsergroupId           types.Binary `json:"usergroup_id"`
}

type NotificationRecipient struct {
	NotificationUser `json:",inline"`
	UsergroupId      types.Binary `json:"usergroup_id"`
}

type NotificationCustomvar struct {
	CustomvarMeta  `json:",inline"`
	NotificationId types.Binary `json:"object_id"`
}

func NewNotification() contracts.Entity {
	return &Notification{}
}

func NewNotificationUser() contracts.Entity {
	return &NotificationUser{}
}

func NewNotificationUsergroup() contracts.Entity {
	return &NotificationUsergroup{}
}

func NewNotificationRecipient() contracts.Entity {
	return &NotificationRecipient{}
}

func NewNotificationCustomvar() contracts.Entity {
	return &NotificationCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Notification)(nil)
)
