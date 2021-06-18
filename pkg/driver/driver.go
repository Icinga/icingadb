package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io/ioutil"
	"log"
	"sync"
	"syscall"
	"time"
)

var timeout = time.Minute * 5

// RetryConnector wraps driver.Connector with retry logic.
type RetryConnector struct {
	driver.Connector
	driver Driver
}

// Connect implements part of the driver.Connector interface.
func (c RetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	var conn driver.Conn
	var logFirstError sync.Once
	err := errors.Wrap(retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			conn, err = c.Connector.Connect(ctx)

			logFirstError.Do(func() {
				if err != nil {
					c.driver.Logger.Warnw("Can't connect to database. Retrying", zap.Error(err))
				}
			})

			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		timeout,
	), "can't connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c RetryConnector) Driver() driver.Driver {
	return c.driver
}

// Driver wraps driver.Driver with logging capabilities and provides our RetryConnector.
type Driver struct {
	driver.Driver
	Logger *zap.SugaredLogger
}

// OpenConnector implements the DriverContext interface.
func (d Driver) OpenConnector(name string) (driver.Connector, error) {
	var connector driver.Connector
	if dc, ok := d.Driver.(driver.DriverContext); ok {
		c, err := dc.OpenConnector(name)
		if err != nil {
			return nil, err
		}
		connector = c
	} else {
		connector = &ctxConnector{
			driver: d.Driver,
			name:   name,
		}
	}

	return &RetryConnector{
		driver: d,
		Connector: connector,
	}, nil
}

func shouldRetry(err error) bool {
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, syscall.ECONNREFUSED) {
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

func Register(logger *zap.SugaredLogger) {
	sql.Register("icingadb-mysql", &Driver{Driver: &mysql.MySQLDriver{}, Logger: logger})
	// TODO(el): Don't discard but hide?
	_ = mysql.SetLogger(log.New(ioutil.Discard, "", 0))
}

// ctxConnector adds driver.DriverContext support for drivers that do not implement it.
type ctxConnector struct {
	driver driver.Driver
	name   string
}

// Connect implements part of the driver.Connector interface.
func (c *ctxConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.driver.Open(c.name)
	if err != nil {
		return nil, err
	}

	select {
	default:
	case <-ctx.Done():
		conn.Close()

		return nil, ctx.Err()
	}

	return conn, nil
}

// Driver implements part of the driver.Connector interface.
func (c *ctxConnector) Driver() driver.Driver {
	return c.driver
}
