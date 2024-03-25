package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"strings"
)

// String adds JSON support to sql.NullString.
type String struct {
	sql.NullString
}

// MakeString constructs a new non-NULL String from s.
func MakeString(s string) String {
	return String{sql.NullString{
		String: s,
		Valid:  true,
	}}
}

// MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (s String) MarshalJSON() ([]byte, error) {
	var v interface{}
	if s.Valid {
		v = s.String
	}

	return internal.MarshalJSON(v)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *String) UnmarshalText(text []byte) error {
	*s = String{sql.NullString{
		String: string(text),
		Valid:  true,
	}}

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (s *String) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	if err := internal.UnmarshalJSON(data, &s.String); err != nil {
		return err
	}

	s.Valid = true

	return nil
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (s String) Value() (driver.Value, error) {
	if !s.Valid {
		return nil, nil
	}

	// PostgreSQL does not allow null bytes in varchar, char and text fields.
	return strings.ReplaceAll(s.String, "\x00", ""), nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = String{}
	_ encoding.TextUnmarshaler = (*String)(nil)
	_ json.Unmarshaler         = (*String)(nil)
	_ driver.Valuer            = String{}
	_ sql.Scanner              = (*String)(nil)
)
