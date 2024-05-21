package config

import (
	stderrors "errors"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
	"reflect"
)

// ErrInvalidArgument is the error returned by [ParseFlags]
// its parsing result cannot be stored in the value pointed to by the designated passed argument which
// must be a non-nil pointer.
var ErrInvalidArgument = stderrors.New("invalid argument")

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

// ParseFlags parses CLI flags and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// ParseFlags returns an [ErrInvalidArgument] error.
// ParseFlags adds a default Help Options group,
// which contains the options -h and --help.
// If either option is specified on the command line,
// ParseFlags prints the help message to [os.Stdout] and exits.
// Note that errors are not printed automatically,
// so error handling is the sole responsibility of the caller.
func ParseFlags(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.Wrapf(ErrInvalidArgument, "non-nil pointer expected, got %T", v)
	}

	parser := flags.NewParser(v, flags.Default^flags.PrintErrors)

	if _, err := parser.Parse(); err != nil {
		var flagErr *flags.Error
		if errors.As(err, &flagErr) && flagErr.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stdout, flagErr)
			os.Exit(0)
		}

		return errors.Wrap(err, "can't parse CLI flags")
	}

	return nil
}
