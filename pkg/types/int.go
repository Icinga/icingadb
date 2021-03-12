package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
)

// Int adds JSON support to sql.NullInt64.
type Int struct {
	sql.NullInt64
}

// MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (i Int) MarshalJSON() ([]byte, error) {
	var v interface{}
	if i.Valid {
		v = i.Int64
	}

	return json.Marshal(v)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (i *Int) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	err := json.Unmarshal(data, &i.Int64)
	if err == nil {
		i.Valid = true
	}

	return err
}

// Assert interface compliance.
var (
	_ json.Marshaler   = Int{}
	_ json.Unmarshaler = (*Int)(nil)
	_ driver.Valuer    = Int{}
	_ sql.Scanner      = (*Int)(nil)
)
