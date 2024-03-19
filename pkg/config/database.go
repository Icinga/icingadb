package config

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	icingadbDriver "github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/lib/pq"
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
		icingadbDriver.Register(logger)
	})

	var db *sqlx.DB
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

		c, err := mysql.NewConnector(config)
		if err != nil {
			return nil, errors.Wrap(err, "can't open mysql database")
		}

		wsrepSyncWait := int64(d.Options.WsrepSyncWait)
		setWsrepSyncWait := func(ctx context.Context, conn driver.Conn) error {
			return setGaleraOpts(ctx, conn, wsrepSyncWait)
		}

		db = sqlx.NewDb(sql.OpenDB(icingadbDriver.NewConnector(c, logger, setWsrepSyncWait)), icingadbDriver.MySQL)
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

		connector, err := pq.NewConnector(uri.String())
		if err != nil {
			return nil, errors.Wrap(err, "can't open pgsql database")
		}

		db = sqlx.NewDb(sql.OpenDB(icingadbDriver.NewConnector(connector, logger, nil)), icingadbDriver.PostgreSQL)
	default:
		return nil, unknownDbType(d.Type)
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

// setGaleraOpts sets the "wsrep_sync_wait" variable for each session ensures that causality checks are performed
// before execution and that each statement is executed on a fully synchronized node. Doing so prevents foreign key
// violation when inserting into dependent tables on different MariaDB/MySQL nodes. When using MySQL single nodes,
// the "SET SESSION" command will fail with "Unknown system variable (1193)" and will therefore be silently dropped.
//
// https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_sync_wait
func setGaleraOpts(ctx context.Context, conn driver.Conn, wsrepSyncWait int64) error {
	const galeraOpts = "SET SESSION wsrep_sync_wait=?"

	stmt, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, galeraOpts)
	if err != nil {
		if errors.Is(err, &mysql.MySQLError{Number: 1193}) { // Unknown system variable
			return nil
		}

		return errors.Wrap(err, "cannot prepare "+galeraOpts)
	}
	// This is just for an unexpected exit and any returned error can safely be ignored and in case
	// of the normal function exit, the stmt is closed manually, and its error is handled gracefully.
	defer func() { _ = stmt.Close() }()

	_, err = stmt.(driver.StmtExecContext).ExecContext(ctx, []driver.NamedValue{{Value: wsrepSyncWait}})
	if err != nil {
		return errors.Wrap(err, "cannot execute "+galeraOpts)
	}

	if err = stmt.Close(); err != nil {
		return errors.Wrap(err, "cannot close prepared statement "+galeraOpts)
	}

	return nil
}
