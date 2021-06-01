package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
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

	return internal.MarshalJSON(v)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (f *Float) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return internal.CantParseFloat64(err, string(text))
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

	if err := internal.UnmarshalJSON(data, &f.Float64); err != nil {
		return err
	}

	f.Valid = true

	return nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = Float{}
	_ encoding.TextUnmarshaler = (*Float)(nil)
	_ json.Unmarshaler         = (*Float)(nil)
	_ driver.Valuer            = Float{}
	_ sql.Scanner              = (*Float)(nil)
)
