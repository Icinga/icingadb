package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
)

// String adds JSON support to sql.NullString.
type String struct {
	sql.NullString
}

// MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (s String) MarshalJSON() ([]byte, error) {
	var v interface{}
	if s.Valid {
		v = s.String
	}

	return json.Marshal(v)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (s *String) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	err := json.Unmarshal(data, &s.String)
	if err == nil {
		s.Valid = true
	}

	return err
}

// Assert interface compliance.
var (
	_ json.Marshaler   = String{}
	_ json.Unmarshaler = (*String)(nil)
	_ driver.Valuer    = String{}
	_ sql.Scanner      = (*String)(nil)
)
