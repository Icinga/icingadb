package config

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
)

type Validator interface {
	Validate() error
}

// FromYAMLFile returns a new value of type T created from the given YAML config file.
func FromYAMLFile[T any, P interface {
	*T
	Validator
}](name string) (*T, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.Wrap(err, "can't open YAML file "+name)
	}
	defer f.Close()

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
// returns a Flags value created from them.
func ParseFlags[T any, P interface{ *T }]() (*T, error) {
	f := P(new(T))
	parser := flags.NewParser(f, flags.Default)

	if _, err := parser.Parse(); err != nil {
		return nil, errors.Wrap(err, "can't parse CLI flags")
	}

	return f, nil
}

// TLS provides TLS configuration options.
type TLS struct {
	Enable   bool   `yaml:"tls"`
	Cert     string `yaml:"cert"`
	Key      string `yaml:"key"`
	Ca       string `yaml:"ca"`
	Insecure bool   `yaml:"insecure"`
}

// MakeConfig assembles a tls.Config from t and serverName.
func (t *TLS) MakeConfig(serverName string) (*tls.Config, error) {
	if !t.Enable {
		return nil, nil
	}

	tlsConfig := &tls.Config{}
	if t.Cert == "" {
		if t.Key != "" {
			return nil, errors.New("private key given, but client certificate missing")
		}
	} else if t.Key == "" {
		return nil, errors.New("client certificate given, but private key missing")
	} else {
		crt, err := tls.LoadX509KeyPair(t.Cert, t.Key)
		if err != nil {
			return nil, errors.Wrap(err, "can't load X.509 key pair")
		}

		tlsConfig.Certificates = []tls.Certificate{crt}
	}

	if t.Insecure {
		tlsConfig.InsecureSkipVerify = true
	} else if t.Ca != "" {
		raw, err := ioutil.ReadFile(t.Ca)
		if err != nil {
			return nil, errors.Wrap(err, "can't read CA file")
		}

		tlsConfig.RootCAs = x509.NewCertPool()
		if !tlsConfig.RootCAs.AppendCertsFromPEM(raw) {
			return nil, errors.New("can't parse CA file")
		}
	}

	tlsConfig.ServerName = serverName

	return tlsConfig, nil
}
