package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"github.com/pkg/errors"
	"strconv"
	"time"
)

// UnixNano is a nullable nanosecond Unix timestamp in databases and JSON.
//
// Please be aware that according to Time.UnixNano's documentation the internal int64 cannot hold a date before the year
// 1678 or after 2262. Using UnixNano creates a Year 2262 problem, unless we will have ditched int64 or gone extinct.
type UnixNano time.Time

// Time returns the time.Time conversion of UnixNano.
func (t UnixNano) Time() time.Time {
	return time.Time(t)
}

// MarshalJSON implements the json.Marshaler interface.
// Marshals to nanoseconds. Supports JSON null.
func (t UnixNano) MarshalJSON() ([]byte, error) {
	if time.Time(t).IsZero() {
		return []byte("null"), nil
	}

	return []byte(strconv.FormatInt(time.Time(t).UnixNano(), 10)), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *UnixNano) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return internal.CantParseFloat64(err, string(text))
	}

	*t = UnixNano(time.Unix(0, int64(parsed)))
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Unmarshals from nanoseconds. Supports JSON null.
func (t *UnixNano) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	ns, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return internal.CantParseFloat64(err, string(data))
	}
	*t = UnixNano(time.Unix(0, int64(ns)))

	return nil
}

// Scan implements the sql.Scanner interface.
// Scans from nanoseconds. Supports SQL NULL.
func (t *UnixNano) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	v, ok := src.(int64)
	if !ok {
		return errors.Errorf("bad int64 type assertion from %#v", src)
	}
	*t = UnixNano(time.Unix(0, v))

	return nil
}

// Value implements the driver.Valuer interface.
// Returns nanoseconds. Supports SQL NULL.
func (t UnixNano) Value() (driver.Value, error) {
	if t.Time().IsZero() {
		return nil, nil
	}

	return t.Time().UnixNano(), nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = (*UnixNano)(nil)
	_ encoding.TextUnmarshaler = (*UnixNano)(nil)
	_ json.Unmarshaler         = (*UnixNano)(nil)
	_ sql.Scanner              = (*UnixNano)(nil)
	_ driver.Valuer            = (*UnixNano)(nil)
)
