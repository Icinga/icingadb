package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"github.com/pkg/errors"
	"strconv"
)

var (
	enum = map[bool]string{
		true:  "y",
		false: "n",
	}
)

// Bool represents a bool for ENUM ('y', 'n'), which can be NULL.
type Bool struct {
	Bool  bool
	Valid bool // Valid is true if Bool is not NULL
}

// MarshalJSON implements the json.Marshaler interface.
func (b Bool) MarshalJSON() ([]byte, error) {
	if !b.Valid {
		return []byte("null"), nil
	}

	return internal.MarshalJSON(b.Bool)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (b *Bool) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseUint(string(text), 10, 64)
	if err != nil {
		return internal.CantParseUint64(err, string(text))
	}

	*b = Bool{parsed != 0, true}
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (b *Bool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	if err := internal.UnmarshalJSON(data, &b.Bool); err != nil {
		return err
	}

	b.Valid = true

	return nil
}

// Scan implements the sql.Scanner interface.
// Supports SQL NULL.
func (b *Bool) Scan(src interface{}) error {
	if src == nil {
		b.Bool, b.Valid = false, false
		return nil
	}

	v, ok := src.([]byte)
	if !ok {
		return errors.Errorf("bad []byte type assertion from %#v", src)
	}

	switch string(v) {
	case "y":
		b.Bool = true
	case "n":
		b.Bool = false
	default:
		return errors.Errorf("bad bool %#v", v)
	}

	b.Valid = true

	return nil
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (b Bool) Value() (driver.Value, error) {
	if !b.Valid {
		return nil, nil
	}

	return enum[b.Bool], nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = (*Bool)(nil)
	_ encoding.TextUnmarshaler = (*Bool)(nil)
	_ json.Unmarshaler         = (*Bool)(nil)
	_ sql.Scanner              = (*Bool)(nil)
	_ driver.Valuer            = (*Bool)(nil)
)
