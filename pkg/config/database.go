package config

import (
	"fmt"
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
)

var registerDriverOnce sync.Once

// Database defines database client configuration.
type Database struct {
	Host     string           `yaml:"host"`
	Port     int              `yaml:"port"`
	Database string           `yaml:"database"`
	User     string           `yaml:"user"`
	Password string           `yaml:"password"`
	Options  icingadb.Options `yaml:"options"`
}

// Open prepares the DSN string and driver configuration,
// calls sqlx.Open, but returns *icingadb.DB.
func (d *Database) Open(logger *zap.SugaredLogger) (*icingadb.DB, error) {
	registerDriverOnce.Do(func() {
		driver.Register(logger)
	})

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?timeout=60s",
		d.User, d.Password, d.Host, d.Port, d.Database)

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

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (d *Database) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(d); err != nil {
		return errors.Wrap(err, "can't set default database config")
	}
	// Prevent recursion.
	type self Database
	if err := unmarshal((*self)(d)); err != nil {
		return internal.CantUnmarshalYAML(err, d)
	}

	return nil
}
