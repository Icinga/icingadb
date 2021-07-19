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
type UnixMilli struct {
	sql.NullInt64
}

// Time returns the time.Time conversion of UnixMilli.
func (t UnixMilli) Time() time.Time {
	return utils.FromUnixMilli(t.Int64)
}

// MarshalJSON implements the json.Marshaler interface.
// Marshals to milliseconds. Supports JSON null.
func (t UnixMilli) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return nil, nil
	}

	return []byte(strconv.FormatInt(t.Int64, 10)), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *UnixMilli) UnmarshalText(text []byte) error {
	ms, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return internal.CantParseFloat64(err, string(text))
	}

	*t = UnixMilli{sql.NullInt64{
		Int64: int64(ms),
		Valid: true,
	}}

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

	*t = UnixMilli{sql.NullInt64{
		Int64: int64(ms),
		Valid: true,
	}}

	return nil
}

// Scan implements the sql.Scanner interface.
// Scans from milliseconds. Supports SQL NULL.
func (t *UnixMilli) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	ms, ok := src.(int64)
	if !ok {
		return errors.Errorf("bad int64 type assertion from %#v", src)
	}

	*t = UnixMilli{sql.NullInt64{
		Int64: ms,
		Valid: true,
	}}

	return nil
}

// Value implements the driver.Valuer interface.
// Returns milliseconds. Supports SQL NULL.
func (t UnixMilli) Value() (driver.Value, error) {
	if !t.Valid {
		return nil, nil
	}

	return t.Int64, nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = (*UnixMilli)(nil)
	_ encoding.TextUnmarshaler = (*UnixMilli)(nil)
	_ json.Unmarshaler         = (*UnixMilli)(nil)
	_ sql.Scanner              = (*UnixMilli)(nil)
	_ driver.Valuer            = (*UnixMilli)(nil)
)
