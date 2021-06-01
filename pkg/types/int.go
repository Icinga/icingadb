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

	return internal.MarshalJSON(v)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (i *Int) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return internal.CantParseInt64(err, string(text))
	}

	*i = Int{sql.NullInt64{
		Int64: parsed,
		Valid: true,
	}}

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (i *Int) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	if err := internal.UnmarshalJSON(data, &i.Int64); err != nil {
		return err
	}

	i.Valid = true

	return nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = Int{}
	_ json.Unmarshaler         = (*Int)(nil)
	_ encoding.TextUnmarshaler = (*Int)(nil)
	_ driver.Valuer            = Int{}
	_ sql.Scanner              = (*Int)(nil)
)
