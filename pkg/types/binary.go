package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/pkg/errors"
)

// Binary nullable byte string. Hex as JSON.
type Binary []byte

// nullBinary for validating whether a Binary is valid.
var nullBinary Binary

// Equal returns whether the binaries are the same length and
// contain the same bytes.
func (binary Binary) Equal(equaler contracts.Equaler) bool {
	b, ok := equaler.(Binary)
	if !ok {
		panic("bad Binary type assertion")
	}

	return bytes.Equal(binary, b)
}

// Valid returns whether the Binary is valid.
func (binary Binary) Valid() bool {
	return !bytes.Equal(binary, nullBinary)
}

// String returns the hex string representation form of the Binary.
func (binary Binary) String() string {
	return hex.EncodeToString(binary)
}

// MarshalText implements a custom marhsal function to encode
// the Binary as hex. MarshalText implements the
// encoding.TextMarshaler interface.
func (binary Binary) MarshalText() ([]byte, error) {
	return []byte(binary.String()), nil
}

// UnmarshalText implements a custom unmarshal function to decode
// hex into a Binary. UnmarshalText implements the
// encoding.TextUnmarshaler interface.
func (binary *Binary) UnmarshalText(text []byte) error {
	b := make([]byte, hex.DecodedLen(len(text)))
	_, err := hex.Decode(b, text)
	if err != nil {
		return internal.CantDecodeHex(err, string(text))
	}
	*binary = b

	return nil
}

// MarshalJSON implements a custom marshal function to encode the Binary
// as a hex string. MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (binary Binary) MarshalJSON() ([]byte, error) {
	if !binary.Valid() {
		return []byte("null"), nil
	}

	return internal.MarshalJSON(binary.String())
}

// UnmarshalJSON implements a custom unmarshal function to decode
// a JSON hex string into a Binary. UnmarshalJSON implements the
// json.Unmarshaler interface. Supports JSON null.
func (binary *Binary) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	var s string
	if err := internal.UnmarshalJSON(data, &s); err != nil {
		return err
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return internal.CantDecodeHex(err, s)
	}
	*binary = b

	return nil
}

// Scan implements the sql.Scanner interface.
// Supports SQL NULL.
func (binary *Binary) Scan(src interface{}) error {
	switch src := src.(type) {
	case nil:
		return nil

	case []byte:
		if len(src) == 0 {
			return nil
		}

		b := make([]byte, len(src))
		copy(b, src)
		*binary = b

	default:
		return errors.Errorf("unable to scan type %T into Binary", src)
	}

	return nil
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (binary Binary) Value() (driver.Value, error) {
	if !binary.Valid() {
		return nil, nil
	}

	return []byte(binary), nil
}

// Assert interface compliance.
var (
	_ contracts.ID             = (*Binary)(nil)
	_ encoding.TextMarshaler   = (*Binary)(nil)
	_ encoding.TextUnmarshaler = (*Binary)(nil)
	_ json.Marshaler           = (*Binary)(nil)
	_ json.Unmarshaler         = (*Binary)(nil)
	_ sql.Scanner              = (*Binary)(nil)
	_ driver.Valuer            = (*Binary)(nil)
)
