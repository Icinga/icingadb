package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
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

	return []byte(strconv.FormatInt(time.Time(t).UnixMilli(), 10)), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *UnixMilli) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return internal.CantParseFloat64(err, string(text))
	}

	*t = UnixMilli(utils.FromUnixMilli(int64(parsed)))
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Unmarshals from milliseconds. Supports JSON null.
func (t *UnixMilli) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	ms, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return internal.CantParseFloat64(err, string(data))
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
		return errors.Errorf("bad int64 type assertion from %#v", src)
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

	return t.Time().UnixMilli(), nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = (*UnixMilli)(nil)
	_ encoding.TextUnmarshaler = (*UnixMilli)(nil)
	_ json.Unmarshaler         = (*UnixMilli)(nil)
	_ sql.Scanner              = (*UnixMilli)(nil)
	_ driver.Valuer            = (*UnixMilli)(nil)
)
