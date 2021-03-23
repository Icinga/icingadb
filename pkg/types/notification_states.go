package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// NotificationStates specifies the set of states a notification may be sent for.
type NotificationStates uint8

// UnmarshalJSON implements the json.Unmarshaler interface.
func (nst *NotificationStates) UnmarshalJSON(bytes []byte) error {
	var states []string
	if err := json.Unmarshal(bytes, &states); err != nil {
		return err
	}

	var n NotificationStates
	for _, state := range states {
		if v, ok := notificationStateNames[state]; ok {
			n |= v
		} else {
			return BadNotificationStates{states}
		}
	}

	*nst = n
	return nil
}

// Value implements the driver.Valuer interface.
func (nst NotificationStates) Value() (driver.Value, error) {
	if nst&^allNotificationStates == 0 {
		return int64(nst), nil
	} else {
		return nil, BadNotificationStates{nst}
	}
}

// BadNotificationStates complains about syntactically, but not semantically valid NotificationStates.
type BadNotificationStates struct {
	States interface{}
}

// Error implements the error interface.
func (bns BadNotificationStates) Error() string {
	return fmt.Sprintf("bad notification states: %#v", bns.States)
}

// notificationStateNames maps all valid NotificationStates values to their SQL representation.
var notificationStateNames = map[string]NotificationStates{
	"OK":       1,
	"Warning":  2,
	"Critical": 4,
	"Unknown":  8,
	"Up":       16,
	"Down":     32,
}

// allNotificationStates is the largest valid NotificationStates value.
var allNotificationStates = func() NotificationStates {
	var nt NotificationStates
	for _, v := range notificationStateNames {
		nt |= v
	}

	return nt
}()

// Assert interface compliance.
var (
	_ error            = BadNotificationStates{}
	_ json.Unmarshaler = (*NotificationStates)(nil)
	_ driver.Valuer    = NotificationStates(0)
)
