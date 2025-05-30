package history

import (
	"database/sql/driver"
	"encoding"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/pkg/errors"
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
	t := string(text)
	for bitset, name := range notificationTypes {
		if t == name || t == bitset {
			*nt = NotificationType(name)
			return nil
		}
	}

	return errors.Errorf("bad notification type: %#v", t)
}

// Value implements the driver.Valuer interface.
func (nt NotificationType) Value() (driver.Value, error) { return string(nt), nil }

// notificationTypes maps all valid NotificationType values to their SQL representation.
// The keys are the bitset values as strings, and the values are the corresponding names.
var notificationTypes = map[string]string{
	"1":   "downtime_start",
	"2":   "downtime_end",
	"4":   "downtime_removed",
	"8":   "custom",
	"16":  "acknowledgement",
	"32":  "problem",
	"64":  "recovery",
	"128": "flapping_start",
	"256": "flapping_end",
}

// Assert interface compliance.
var (
	_ UpserterEntity           = (*NotificationHistory)(nil)
	_ UpserterEntity           = (*UserNotificationHistory)(nil)
	_ database.TableNamer      = (*HistoryNotification)(nil)
	_ UpserterEntity           = (*HistoryNotification)(nil)
	_ encoding.TextUnmarshaler = (*NotificationType)(nil)
	_ driver.Valuer            = NotificationType("")
)
