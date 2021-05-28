package types

import (
	"database/sql/driver"
	"encoding"
	"fmt"
	"strconv"
)

// NotificationType specifies the reason of a sent notification.
type NotificationType uint16

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (nt *NotificationType) UnmarshalText(bytes []byte) error {
	text := string(bytes)

	i, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return err
	}

	n := NotificationType(i)
	if uint64(n) != i {
		// Truncated due to above cast, obviously too high
		return BadNotificationType{text}
	}

	if _, ok := notificationTypes[n]; !ok {
		return BadNotificationType{text}
	}

	*nt = n
	return nil
}

// Value implements the driver.Valuer interface.
func (nt NotificationType) Value() (driver.Value, error) {
	if v, ok := notificationTypes[nt]; ok {
		return v, nil
	} else {
		return nil, BadNotificationType{nt}
	}
}

// BadNotificationType complains about a syntactically, but not semantically valid NotificationType.
type BadNotificationType struct {
	Type interface{}
}

// Error implements the error interface.
func (bnt BadNotificationType) Error() string {
	return fmt.Sprintf("bad notification type: %#v", bnt.Type)
}

// notificationTypes maps all valid NotificationType values to their SQL representation.
var notificationTypes = map[NotificationType]string{
	1:   "downtime_start",
	2:   "downtime_end",
	4:   "downtime_removed",
	8:   "custom",
	16:  "acknowledgement",
	32:  "problem",
	64:  "recovery",
	128: "flapping_start",
	256: "flapping_end",
}

// Assert interface compliance.
var (
	_ error                    = BadNotificationType{}
	_ encoding.TextUnmarshaler = (*NotificationType)(nil)
	_ driver.Valuer            = NotificationType(0)
)
