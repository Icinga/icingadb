package config

import (
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
)

// FromYAMLFile returns a new value of type T created from the given YAML config file.
func FromYAMLFile[T any, P interface {
	*T
	Validator
}](name string) (*T, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.Wrap(err, "can't open YAML file "+name)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	c := P(new(T))

	if err := defaults.Set(c); err != nil {
		return nil, errors.Wrap(err, "can't set config defaults")
	}

	d := yaml.NewDecoder(f, yaml.DisallowUnknownField())
	if err := d.Decode(c); err != nil {
		return nil, errors.Wrap(err, "can't parse YAML file "+name)
	}

	if err := c.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	return c, nil
}

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
