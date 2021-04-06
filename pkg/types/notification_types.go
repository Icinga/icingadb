package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// NotificationTypes specifies the set of reasons a notification may be sent for.
type NotificationTypes uint16

// UnmarshalJSON implements the json.Unmarshaler interface.
func (nt *NotificationTypes) UnmarshalJSON(bytes []byte) error {
	var types []string
	if err := json.Unmarshal(bytes, &types); err != nil {
		return err
	}

	var n NotificationTypes
	for _, typ := range types {
		if v, ok := notificationTypeNames[typ]; ok {
			n |= v
		} else {
			return BadNotificationTypes{types}
		}
	}

	*nt = n
	return nil
}

// Value implements the driver.Valuer interface.
func (nt NotificationTypes) Value() (driver.Value, error) {
	if nt&^allNotificationTypes == 0 {
		return int64(nt), nil
	} else {
		return nil, BadNotificationTypes{nt}
	}
}

// BadNotificationTypes complains about syntactically, but not semantically valid NotificationTypes.
type BadNotificationTypes struct {
	Types interface{}
}

// Error implements the error interface.
func (bnt BadNotificationTypes) Error() string {
	return fmt.Sprintf("bad notification types: %#v", bnt.Types)
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
	_ error            = BadNotificationTypes{}
	_ json.Unmarshaler = (*NotificationTypes)(nil)
	_ driver.Valuer    = NotificationTypes(0)
)
