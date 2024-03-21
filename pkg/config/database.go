package config

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/pkg/errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var registerDriverOnce sync.Once

// Database defines database client configuration.
type Database struct {
	Type       string           `yaml:"type" default:"mysql"`
	Host       string           `yaml:"host"`
	Port       int              `yaml:"port"`
	Database   string           `yaml:"database"`
	User       string           `yaml:"user"`
	Password   string           `yaml:"password"`
	TlsOptions TLS              `yaml:",inline"`
	Options    icingadb.Options `yaml:"options"`
}

// Open prepares the DSN string and driver configuration,
// calls sqlx.Open, but returns *icingadb.DB.
func (d *Database) Open(logger *logging.Logger) (*icingadb.DB, error) {
	registerDriverOnce.Do(func() {
		driver.Register(logger)
	})

	var dsn string
	switch d.Type {
	case "mysql":
		config := mysql.NewConfig()

		config.User = d.User
		config.Passwd = d.Password

		if d.isUnixAddr() {
			config.Net = "unix"
			config.Addr = d.Host
		} else {
			config.Net = "tcp"
			port := d.Port
			if port == 0 {
				port = 3306
			}
			config.Addr = net.JoinHostPort(d.Host, fmt.Sprint(port))
		}

		config.DBName = d.Database
		config.Timeout = time.Minute
		config.Params = map[string]string{"sql_mode": "'TRADITIONAL,ANSI_QUOTES'"}

		tlsConfig, err := d.TlsOptions.MakeConfig(d.Host)
		if err != nil {
			return nil, err
		}

		if tlsConfig != nil {
			config.TLSConfig = "icingadb"
			if err := mysql.RegisterTLSConfig(config.TLSConfig, tlsConfig); err != nil {
				return nil, errors.Wrap(err, "can't register TLS config")
			}
		}

		dsn = config.FormatDSN()
	case "pgsql":
		uri := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(d.User, d.Password),
			Path:   "/" + url.PathEscape(d.Database),
		}

		query := url.Values{
			"connect_timeout":   {"60"},
			"binary_parameters": {"yes"},

			// Host and port can alternatively be specified in the query string. lib/pq can't parse the connection URI
			// if a Unix domain socket path is specified in the host part of the URI, therefore always use the query
			// string. See also https://github.com/lib/pq/issues/796
			"host": {d.Host},
		}
		if d.Port != 0 {
			query["port"] = []string{strconv.FormatInt(int64(d.Port), 10)}
		}

		if _, err := d.TlsOptions.MakeConfig(d.Host); err != nil {
			return nil, err
		}

		if d.TlsOptions.Enable {
			if d.TlsOptions.Insecure {
				query["sslmode"] = []string{"require"}
			} else {
				query["sslmode"] = []string{"verify-full"}
			}

			if d.TlsOptions.Cert != "" {
				query["sslcert"] = []string{d.TlsOptions.Cert}
			}

			if d.TlsOptions.Key != "" {
				query["sslkey"] = []string{d.TlsOptions.Key}
			}

			if d.TlsOptions.Ca != "" {
				query["sslrootcert"] = []string{d.TlsOptions.Ca}
			}
		} else {
			query["sslmode"] = []string{"disable"}
		}

		uri.RawQuery = query.Encode()
		dsn = uri.String()
	default:
		return nil, unknownDbType(d.Type)
	}

	db, err := sqlx.Open("icingadb-"+d.Type, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "can't open database")
	}

	db.SetMaxIdleConns(d.Options.MaxConnections / 3)
	db.SetMaxOpenConns(d.Options.MaxConnections)

	db.Mapper = reflectx.NewMapperFunc("db", func(s string) string {
		return utils.Key(s, '_')
	})

	return icingadb.NewDb(db, logger, &d.Options), nil
}

// Validate checks constraints in the supplied database configuration and returns an error if they are violated.
func (d *Database) Validate() error {
	switch d.Type {
	case "mysql", "pgsql":
	default:
		return unknownDbType(d.Type)
	}

	if d.Host == "" {
		return errors.New("database host missing")
	}

	if d.User == "" {
		return errors.New("database user missing")
	}

	if d.Database == "" {
		return errors.New("database name missing")
	}

	return d.Options.Validate()
}

func (d *Database) isUnixAddr() bool {
	return strings.HasPrefix(d.Host, "/")
}

func unknownDbType(t string) error {
	return errors.Errorf(`unknown database type %q, must be one of: "mysql", "pgsql"`, t)
}
