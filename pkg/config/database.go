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
	"sync"
	"time"
)

var registerDriverOnce sync.Once

// Database defines database client configuration.
type Database struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	TLS      `yaml:",inline"`
	Options  icingadb.Options `yaml:"options"`
}

// Open prepares the DSN string and driver configuration,
// calls sqlx.Open, but returns *icingadb.DB.
func (d *Database) Open(logger *logging.Logger) (*icingadb.DB, error) {
	registerDriverOnce.Do(func() {
		driver.Register(logger)
	})

	config := mysql.NewConfig()

	config.User = d.User
	config.Passwd = d.Password
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(d.Host, fmt.Sprint(d.Port))
	config.DBName = d.Database
	config.Timeout = time.Minute

	tlsConfig, err := d.TLS.MakeConfig(config.Addr)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		config.TLSConfig = "icingadb"
		if err := mysql.RegisterTLSConfig(config.TLSConfig, tlsConfig); err != nil {
			return nil, errors.Wrap(err, "can't register TLS config")
		}
	}

	dsn := config.FormatDSN()

	db, err := sqlx.Open("icingadb-mysql", dsn)
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
	return d.Options.Validate()
}
