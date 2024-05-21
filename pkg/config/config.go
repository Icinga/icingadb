package config

import (
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
)

// ParseFlags parses CLI flags and
// returns a new value of type T created from them.
func ParseFlags[T any]() (*T, error) {
	var f T

	parser := flags.NewParser(&f, flags.Default)

	if _, err := parser.Parse(); err != nil {
		return nil, errors.Wrap(err, "can't parse CLI flags")
	}

	return &f, nil
}
