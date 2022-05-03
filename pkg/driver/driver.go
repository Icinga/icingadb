package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"syscall"
	"time"
)

const MySQL = "icingadb-mysql"
const PostgreSQL = "icingadb-pgsql"

var timeout = time.Minute * 5

// RetryConnector wraps driver.Connector with retry logic.
type RetryConnector struct {
	driver.Connector
	driver Driver
}

// Connect implements part of the driver.Connector interface.
func (c RetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	var conn driver.Conn
	err := errors.Wrap(retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			conn, err = c.Connector.Connect(ctx)
			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		retry.Settings{
			Timeout: timeout,
			OnError: func(_ time.Duration, _ uint64, err, lastErr error) {
				if lastErr == nil || err.Error() != lastErr.Error() {
					c.driver.Logger.Warnw("Can't connect to database. Retrying", zap.Error(err))
				}
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, _ error) {
				if attempt > 0 {
					c.driver.Logger.Infow("Reconnected to database",
						zap.Duration("after", elapsed), zap.Uint64("attempts", attempt+1))
				}
			},
		},
	), "can't connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c RetryConnector) Driver() driver.Driver {
	return c.driver
}

// Driver wraps a driver.Driver that also must implement driver.DriverContext with logging capabilities and provides our RetryConnector.
type Driver struct {
	ctxDriver
	Logger *logging.Logger
}

// OpenConnector implements the DriverContext interface.
func (d Driver) OpenConnector(name string) (driver.Connector, error) {
	c, err := d.ctxDriver.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return &RetryConnector{
		driver:    d,
		Connector: c,
	}, nil
}

// Register makes our database Driver available under the name "icingadb-*sql".
func Register(logger *logging.Logger) {
	sql.Register(MySQL, &Driver{ctxDriver: &mysql.MySQLDriver{}, Logger: logger})
	sql.Register(PostgreSQL, &Driver{ctxDriver: PgSQLDriver{}, Logger: logger})
	_ = mysql.SetLogger(mysqlLogger(func(v ...interface{}) { logger.Debug(v...) }))
	sqlx.BindDriver(PostgreSQL, sqlx.DOLLAR)
}

// ctxDriver helps ensure that we only support drivers that implement driver.Driver and driver.DriverContext.
type ctxDriver interface {
	driver.Driver
	driver.DriverContext
}

// mysqlLogger is an adapter that allows ordinary functions to be used as a logger for mysql.SetLogger.
type mysqlLogger func(v ...interface{})

// Print implements the mysql.Logger interface.
func (log mysqlLogger) Print(v ...interface{}) {
	log(v)
}

func shouldRetry(err error) bool {
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EAGAIN) {
		return true
	}

	type temporary interface {
		Temporary() bool
	}
	if t := temporary(nil); errors.As(err, &t) {
		return t.Temporary()
	}

	type timeout interface {
		Timeout() bool
	}
	if t := timeout(nil); errors.As(err, &t) {
		return t.Timeout()
	}

	return false
}
