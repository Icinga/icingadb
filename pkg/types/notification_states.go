package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"github.com/pkg/errors"
)

// NotificationStates specifies the set of states a notification may be sent for.
type NotificationStates uint8

// UnmarshalJSON implements the json.Unmarshaler interface.
func (nst *NotificationStates) UnmarshalJSON(bytes []byte) error {
	var states []string
	if err := internal.UnmarshalJSON(bytes, &states); err != nil {
		return err
	}

	var n NotificationStates
	for _, state := range states {
		if v, ok := notificationStateNames[state]; ok {
			n |= v
		} else {
			return badNotificationStates(states)
		}
	}

	*nst = n
	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (nst *NotificationStates) UnmarshalText(text []byte) error {
	return nst.UnmarshalJSON(text)
}

// Value implements the driver.Valuer interface.
func (nst NotificationStates) Value() (driver.Value, error) {
	if nst&^allNotificationStates == 0 {
		return int64(nst), nil
	} else {
		return nil, badNotificationStates(nst)
	}
}

// badNotificationStates returns an error about syntactically, but not semantically valid NotificationStates.
func badNotificationStates(s interface{}) error {
	return errors.Errorf("bad notification states: %#v", s)
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
	_ json.Unmarshaler         = (*NotificationStates)(nil)
	_ encoding.TextUnmarshaler = (*NotificationStates)(nil)
	_ driver.Valuer            = NotificationStates(0)
)
