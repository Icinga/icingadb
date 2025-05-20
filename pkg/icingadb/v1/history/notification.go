package history

import (
	"database/sql/driver"
	"encoding"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
)

type NotificationHistory struct {
	HistoryTableEntity `json:",inline"`
	HistoryTableMeta   `json:",inline"`
	NotificationId     types.Binary     `json:"notification_id"`
	Type               NotificationType `json:"type"`
	SendTime           types.UnixMilli  `json:"send_time"`
	State              uint8            `json:"state"`
	PreviousHardState  uint8            `json:"previous_hard_state"`
	Author             string           `json:"author"`
	Text               types.String     `json:"text"`
	UsersNotified      uint16           `json:"users_notified"`
}

type UserNotificationHistory struct {
	v1.EntityWithoutChecksum `json:",inline"`
	v1.EnvironmentMeta       `json:",inline"`
	NotificationHistoryId    types.Binary `json:"notification_history_id"`
	UserId                   types.Binary `json:"user_id"`
}

func (u *UserNotificationHistory) Upsert() interface{} {
	return u
}

type HistoryNotification struct {
	HistoryMeta           `json:",inline"`
	NotificationHistoryId types.Binary    `json:"id"`
	EventTime             types.UnixMilli `json:"send_time"`
}

// TableName implements the contracts.TableNamer interface.
func (*HistoryNotification) TableName() string {
	return "history"
}

// NotificationType represents the type of notification for a notification history entry.
//
// Starting with Icinga 2 v2.15, the type is will always be written to Redis as a string.
// This merely exists to provide a compatibility with older history entries lying around in Redis,
// which may have been written as an integer.
type NotificationType string

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (nt *NotificationType) UnmarshalText(text []byte) error {
	switch t := string(text); t {
	case "1":
		*nt = "downtime_start"
	case "2":
		*nt = "downtime_end"
	case "4":
		*nt = "downtime_removed"
	case "8":
		*nt = "custom"
	case "16":
		*nt = "acknowledgement"
	case "32":
		*nt = "problem"
	case "64":
		*nt = "recovery"
	case "128":
		*nt = "flapping_start"
	case "256":
		*nt = "flapping_end"
	default:
		*nt = NotificationType(t)
	}

	return nil
}

// Value implements the driver.Valuer interface.
func (nt NotificationType) Value() (driver.Value, error) { return string(nt), nil }

// Assert interface compliance.
var (
	_ UpserterEntity           = (*NotificationHistory)(nil)
	_ UpserterEntity           = (*UserNotificationHistory)(nil)
	_ database.TableNamer      = (*HistoryNotification)(nil)
	_ UpserterEntity           = (*HistoryNotification)(nil)
	_ encoding.TextUnmarshaler = (*NotificationType)(nil)
	_ driver.Valuer            = NotificationType("")
)
