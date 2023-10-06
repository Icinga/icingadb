package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/pkg/errors"
)

// NotificationTypes specifies the set of reasons a notification may be sent for.
type NotificationTypes uint16

// UnmarshalJSON implements the json.Unmarshaler interface.
func (nt *NotificationTypes) UnmarshalJSON(data []byte) error {
	var types []string
	if err := UnmarshalJSON(data, &types); err != nil {
		return err
	}

	var n NotificationTypes
	for _, typ := range types {
		if v, ok := notificationTypeNames[typ]; ok {
			n |= v
		} else {
			return badNotificationTypes(nt)
		}
	}

	*nt = n
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

// notificationTypeNames maps all valid NotificationTypes values to their SQL representation.
var notificationTypeNames = map[string]NotificationTypes{
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
	var nt NotificationTypes
	for _, v := range notificationTypeNames {
		nt |= v
	}

	return nt
}()

// Assert interface compliance.
var (
	_ json.Unmarshaler         = (*NotificationTypes)(nil)
	_ encoding.TextUnmarshaler = (*NotificationTypes)(nil)
	_ driver.Valuer            = NotificationTypes(0)
)
