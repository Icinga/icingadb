package icingadb

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

// Driver names as automatically registered in the database/sql package by themselves.
const (
	MySQL      string = "mysql"
	PostgreSQL string = "postgres"
)

type InitConnFunc func(context.Context, driver.Conn) error

// RetryConnector wraps driver.Connector with retry logic.
type RetryConnector struct {
	driver.Connector

	logger *logging.Logger

	// initConn can be used to execute post Connect() arbitrary actions.
	// It will be called after successfully initiated a new connection using the connector's Connect method.
	initConn InitConnFunc
}

// NewConnector creates a fully initialized RetryConnector from the given args.
func NewConnector(c driver.Connector, logger *logging.Logger, init InitConnFunc) *RetryConnector {
	return &RetryConnector{Connector: c, logger: logger, initConn: init}
}

// Connect implements part of the driver.Connector interface.
func (c RetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	var conn driver.Conn
	err := errors.Wrap(retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			conn, err = c.Connector.Connect(ctx)
			if err == nil && c.initConn != nil {
				if err = c.initConn(ctx, conn); err != nil {
					// We're going to retry this, so just don't bother whether Close() fails!
					_ = conn.Close()
				}
			}

			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		retry.Settings{
			Timeout: time.Minute * 5,
			OnError: func(_ time.Duration, _ uint64, err, lastErr error) {
				telemetry.UpdateCurrentDbConnErr(err)

				if lastErr == nil || err.Error() != lastErr.Error() {
					c.logger.Warnw("Can't connect to database. Retrying", zap.Error(err))
				}
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, _ error) {
				telemetry.UpdateCurrentDbConnErr(nil)

				if attempt > 0 {
					c.logger.Infow("Reconnected to database",
						zap.Duration("after", elapsed), zap.Uint64("attempts", attempt+1))
				}
			},
		},
	), "can't connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c RetryConnector) Driver() driver.Driver {
	return c.Connector.Driver()
}

// Register sets the default mysql logger to the given one.
func Register(logger *logging.Logger) {
	_ = mysql.SetLogger(mysqlLogger(logger.Debug))
}

// mysqlLogger is an adapter that allows ordinary functions to be used as a logger for mysql.SetLogger.
type mysqlLogger func(v ...interface{})

// Print implements the mysql.Logger interface.
func (log mysqlLogger) Print(v ...interface{}) {
	log(v)
}

func shouldRetry(err error) bool {
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	return retry.Retryable(err)
}
