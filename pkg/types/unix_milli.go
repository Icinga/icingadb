package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/pkg/errors"
	"math"
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
		return []byte("null"), nil
	}

	return []byte(strconv.FormatInt(t.Time().UnixMilli(), 10)), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Unmarshals from milliseconds. Supports JSON null.
func (t *UnixMilli) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		return nil
	}

	return t.fromByteString(data)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (t UnixMilli) MarshalText() ([]byte, error) {
	if time.Time(t).IsZero() {
		return []byte{}, nil
	}

	return []byte(strconv.FormatInt(t.Time().UnixMilli(), 10)), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *UnixMilli) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	return t.fromByteString(text)
}

// Scan implements the sql.Scanner interface.
// Scans from milliseconds. Supports SQL NULL.
func (t *UnixMilli) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return t.fromByteString(v)
	// https://github.com/go-sql-driver/mysql/pull/1452
	case uint64:
		if v > math.MaxInt64 {
			return errors.Errorf("value %v out of range for int64", v)
		}

		*t = UnixMilli(time.UnixMilli(int64(v)))
	case int64:
		*t = UnixMilli(time.UnixMilli(v))
	default:
		return errors.Errorf("bad (u)int64/[]byte type assertion from %[1]v (%[1]T)", src)
	}

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

func (t *UnixMilli) fromByteString(data []byte) error {
	i, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return CantParseInt64(err, string(data))
	}

	*t = UnixMilli(time.UnixMilli(i))

	return nil
}

// Assert interface compliance.
var (
	_ encoding.TextMarshaler   = (*UnixMilli)(nil)
	_ encoding.TextUnmarshaler = (*UnixMilli)(nil)
	_ json.Marshaler           = (*UnixMilli)(nil)
	_ json.Unmarshaler         = (*UnixMilli)(nil)
	_ driver.Valuer            = (*UnixMilli)(nil)
	_ sql.Scanner              = (*UnixMilli)(nil)
)
