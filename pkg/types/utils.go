package types

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"strings"
)

// Name returns the declared name of type t.
func Name(t any) string {
	s := strings.TrimLeft(fmt.Sprintf("%T", t), "*")

	return s[strings.LastIndex(s, ".")+1:]
}

// CantDecodeHex wraps the given error with the given string that cannot be hex-decoded.
func CantDecodeHex(err error, s string) error {
	return errors.Wrapf(err, "can't decode hex %q", s)
}

// CantParseFloat64 wraps the given error with the specified string that cannot be parsed into float64.
func CantParseFloat64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into float64", s)
}

// CantParseInt64 wraps the given error with the specified string that cannot be parsed into int64.
func CantParseInt64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into int64", s)
}

// CantParseUint64 wraps the given error with the specified string that cannot be parsed into uint64.
func CantParseUint64(err error, s string) error {
	return errors.Wrapf(err, "can't parse %q into uint64", s)
}

// CantUnmarshalYAML wraps the given error with the designated value, which cannot be unmarshalled into.
func CantUnmarshalYAML(err error, v interface{}) error {
	return errors.Wrapf(err, "can't unmarshal YAML into %T", v)
}

// MarshalJSON calls json.Marshal and wraps any resulting errors.
func MarshalJSON(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)

	return b, errors.Wrapf(err, "can't marshal JSON from %T", v)
}

// UnmarshalJSON calls json.Unmarshal and wraps any resulting errors.
func UnmarshalJSON(data []byte, v interface{}) error {
	return errors.Wrapf(json.Unmarshal(data, v), "can't unmarshal JSON into %T", v)
}

// Zero returns the zero value for type T.
func Zero[T any]() T {
	return *new(T)
}
