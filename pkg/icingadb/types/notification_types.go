package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/pkg/errors"
)

// NotificationTypes specifies the set of reasons a notification may be sent for.
type NotificationTypes uint16

// UnmarshalJSON implements the json.Unmarshaler interface.
func (nt *NotificationTypes) UnmarshalJSON(data []byte) error {
	var names []string
	if err := types.UnmarshalJSON(data, &names); err != nil {
		return err
	}

	var v NotificationTypes
	for _, name := range names {
		if i, ok := notificationTypeMap[name]; ok {
			v |= i
		} else {
			return badNotificationTypes(nt)
		}
	}

	*nt = v
	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (nt *NotificationTypes) UnmarshalText(text []byte) error {
	return nt.UnmarshalJSON(text)
}

// Value implements the driver.Valuer interface.
func (nt NotificationTypes) Value() (driver.Value, error) {
	if nt&^allNotificationTypes == 0 {
		return int64(nt), nil
	} else {
		return nil, badNotificationTypes(nt)
	}
}

// badNotificationTypes returns an error about syntactically, but not semantically valid NotificationTypes.
func badNotificationTypes(t interface{}) error {
	return errors.Errorf("bad notification types: %#v", t)
}

// notificationTypeMap maps all valid NotificationTypes values to their SQL representation.
var notificationTypeMap = map[string]NotificationTypes{
	"DowntimeStart":   1,
	"DowntimeEnd":     2,
	"DowntimeRemoved": 4,
	"Custom":          8,
	"Acknowledgement": 16,
	"Problem":         32,
	"Recovery":        64,
	"FlappingStart":   128,
	"FlappingEnd":     256,
}

// allNotificationTypes is the largest valid NotificationTypes value.
var allNotificationTypes = func() NotificationTypes {
	var all NotificationTypes
	for _, i := range notificationTypeMap {
		all |= i
	}

	return all
}()

// Assert interface compliance.
var (
	_ json.Unmarshaler         = (*NotificationTypes)(nil)
	_ encoding.TextUnmarshaler = (*NotificationTypes)(nil)
	_ driver.Valuer            = NotificationTypes(0)
)
