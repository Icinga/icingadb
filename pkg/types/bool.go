package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
)

var (
	Yes = Bool{
		Bool:  true,
		Valid: true,
	}

	No = Bool{
		Bool:  false,
		Valid: true,
	}
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
		return nil, nil
	}

	return json.Marshal(b.Bool)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (b *Bool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &b.Bool); err != nil {
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
		return errors.New("bad []byte type assertion")
	}

	switch string(v) {
	case "y":
		b.Bool = true
	case "n":
		b.Bool = false
	default:
		return errors.New("bad bool")
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
	_ json.Marshaler   = (*Bool)(nil)
	_ json.Unmarshaler = (*Bool)(nil)
	_ sql.Scanner      = (*Bool)(nil)
	_ driver.Valuer    = (*Bool)(nil)
)
