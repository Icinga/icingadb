package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type User struct {
	EntityWithChecksum   `json:",inline"`
	EnvironmentMeta      `json:",inline"`
	NameCiMeta           `json:",inline"`
	DisplayName          string       `json:"display_name"`
	Email                string       `json:"email"`
	Pager                string       `json:"pager"`
	NotificationsEnabled types.Bool   `json:"notifications_enabled"`
	TimeperiodId         types.Binary `json:"timeperiod_id"`
	States               uint8        `json:"states"`
	Types                uint16       `json:"types"`
	ZoneId               types.Binary `json:"zone_id"`
}

type UserCustomvar struct {
	CustomvarMeta `json:",inline"`
	UserId        types.Binary `json:"object_id"`
}

func NewUser() contracts.Entity {
	return &User{}
}

func NewUserCustomvar() contracts.Entity {
	return &UserCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*User)(nil)
)
