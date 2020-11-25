package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"github.com/icinga/icingadb/pkg/utils"
	"strconv"
	"time"
)

// UnixMilli is a nullable millisecond UNIX timestamp in databases and JSON.
type UnixMilli time.Time

// Time returns the time.Time conversion of UnixMilli.
func (t UnixMilli) Time() time.Time {
	return time.Time(t)
}

// MarshalJSON implements the json.Marshaler interface.
// Marshals to milliseconds. Supports JSON null.
func (t UnixMilli) MarshalJSON() ([]byte, error) {
	if time.Time(t).IsZero() {
		return nil, nil
	}

	return []byte(strconv.FormatInt(utils.UnixMilli(time.Time(t)), 10)), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Unmarshals from milliseconds. Supports JSON null.
func (t *UnixMilli) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	ms, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	tt := utils.FromUnixMilli(int64(ms))
	*t = UnixMilli(tt)

	return nil
}

// Scan implements the sql.Scanner interface.
// Scans from milliseconds. Supports SQL NULL.
func (t *UnixMilli) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	v, ok := src.(int64)
	if !ok {
		return errors.New("bad int64 type assertion")
	}
	tt := utils.FromUnixMilli(v)
	*t = UnixMilli(tt)

	return nil
}

// Value implements the driver.Valuer interface.
// Returns milliseconds. Supports SQL NULL.
func (t UnixMilli) Value() (driver.Value, error) {
	if t.Time().IsZero() {
		return nil, nil
	}

	return utils.UnixMilli(t.Time()), nil
}

// Assert interface compliance.
var (
	_ json.Marshaler   = (*UnixMilli)(nil)
	_ json.Unmarshaler = (*UnixMilli)(nil)
	_ sql.Scanner      = (*UnixMilli)(nil)
	_ driver.Valuer    = (*UnixMilli)(nil)
)
