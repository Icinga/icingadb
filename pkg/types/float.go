package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"strconv"
)

// Float adds JSON support to sql.NullFloat64.
type Float struct {
	sql.NullFloat64
}

// MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (f Float) MarshalJSON() ([]byte, error) {
	var v interface{}
	if f.Valid {
		v = f.Float64
	}

	return json.Marshal(v)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (f *Float) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return err
	}

	*f = Float{sql.NullFloat64{
		Float64: parsed,
		Valid:   true,
	}}

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (f *Float) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	err := json.Unmarshal(data, &f.Float64)
	if err == nil {
		f.Valid = true
	}

	return err
}

// Assert interface compliance.
var (
	_ json.Marshaler           = Float{}
	_ encoding.TextUnmarshaler = (*Float)(nil)
	_ json.Unmarshaler         = (*Float)(nil)
	_ driver.Valuer            = Float{}
	_ sql.Scanner              = (*Float)(nil)
)
