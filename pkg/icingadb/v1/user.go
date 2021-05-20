package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type User struct {
	EntityWithChecksum   `json:",inline"`
	EnvironmentMeta      `json:",inline"`
	NameCiMeta           `json:",inline"`
	DisplayName          string                   `json:"display_name"`
	Email                string                   `json:"email"`
	Pager                string                   `json:"pager"`
	NotificationsEnabled types.Bool               `json:"notifications_enabled"`
	TimeperiodId         types.Binary             `json:"timeperiod_id"`
	States               types.NotificationStates `json:"states"`
	Types                types.NotificationTypes  `json:"types"`
	ZoneId               types.Binary             `json:"zone_id"`
}

type UserCustomvar struct {
	CustomvarMeta `json:",inline"`
	UserId        types.Binary `json:"user_id"`
}

type Usergroup struct {
	GroupMeta `json:",inline"`
}

type UsergroupCustomvar struct {
	CustomvarMeta `json:",inline"`
	UsergroupId   types.Binary `json:"usergroup_id"`
}

type UsergroupMember struct {
	MemberMeta  `json:",inline"`
	UserId      types.Binary `json:"user_id"`
	UsergroupId types.Binary `json:"usergroup_id"`
}

func NewUser() contracts.Entity {
	return &User{}
}

func NewUserCustomvar() contracts.Entity {
	return &UserCustomvar{}
}

func NewUsergroup() contracts.Entity {
	return &Usergroup{}
}

func NewUsergroupCustomvar() contracts.Entity {
	return &UsergroupCustomvar{}
}

func NewUsergroupMember() contracts.Entity {
	return &UsergroupMember{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*User)(nil)
	_ contracts.Initer = (*Usergroup)(nil)
)
