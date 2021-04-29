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
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

var timeout = time.Minute * 5

// TODO(el): Support DriverContext.
type Driver struct {
	Driver driver.Driver
	Logger *zap.SugaredLogger
}

// TODO(el): Test DNS.
func (d Driver) Open(dsn string) (c driver.Conn, err error) {
	var logFirstError sync.Once
	err = retry.WithBackoff(
		context.Background(),
		func(context.Context) (err error) {
			c, err = d.Driver.Open(dsn)
			logFirstError.Do(func() {
				if err != nil {
					d.Logger.Warnw("Can't connect to database. Retrying", zap.Error(err))
				}
			})
			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		timeout,
	)
	if err != nil {
		err = errors.Wrap(err, "can't connect to database")
	}
	return
}

func shouldRetry(err error) bool {
	underlying := errors.Unwrap(err)
	if underlying == nil {
		underlying = err
	}
	if op, ok := underlying.(*net.OpError); ok {
		underlying = op.Err
	}
	if sys, ok := underlying.(*os.SyscallError); ok {
		underlying = sys.Err
	}
	switch underlying {
	case driver.ErrBadConn, syscall.ECONNREFUSED:
		return true
	}

	type temporary interface {
		Temporary() bool
	}
	if t, ok := underlying.(temporary); ok {
		return t.Temporary()
	}

	type timeout interface {
		Timeout() bool
	}
	if t, ok := underlying.(timeout); ok {
		return t.Timeout()
	}

	return false
}

func Register(logger *zap.SugaredLogger) {
	sql.Register("icingadb-mysql", &Driver{Driver: &mysql.MySQLDriver{}, Logger: logger})
	// TODO(el): Don't discard but hide?
	_ = mysql.SetLogger(log.New(ioutil.Discard, "", 0))
}
