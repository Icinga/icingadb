package history

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type NotificationHistory struct {
	HistoryTableEntity `json:",inline"`
	HistoryTableMeta   `json:",inline"`
	NotificationId     types.Binary           `json:"notification_id"`
	Type               types.NotificationType `json:"type"`
	SendTime           types.UnixMilli        `json:"send_time"`
	State              uint8                  `json:"state"`
	PreviousHardState  uint8                  `json:"previous_hard_state"`
	Author             string                 `json:"author"`
	Text               string                 `json:"text"`
	UsersNotified      uint16                 `json:"users_notified"`
}

type UserNotificationHistory struct {
	HistoryTableEntity    `json:",inline"`
	EnvironmentId         types.Binary `json:"environment_id"`
	NotificationHistoryId types.UUID   `json:"notification_history_id"`
	UserId                types.Binary `json:"user_id"`
}

type HistoryNotification struct {
	HistoryMeta           `json:",inline"`
	NotificationHistoryId types.UUID      `json:"id"`
	EventTime             types.UnixMilli `json:"send_time"`
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryNotification) TableName() string {
	return "history"
}

// Assert interface compliance.
var (
	_ UpserterEntity       = (*NotificationHistory)(nil)
	_ UpserterEntity       = (*UserNotificationHistory)(nil)
	_ contracts.TableNamer = (*HistoryNotification)(nil)
	_ UpserterEntity       = (*HistoryNotification)(nil)
)
