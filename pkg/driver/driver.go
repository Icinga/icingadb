package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/backoff"
	"github.com/icinga/icingadb/pkg/retry"
	"go.uber.org/zap"
	"io/ioutil"
	"log"
	"net"
	"os"
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
	err = retry.WithBackoff(
		context.Background(),
		func(context.Context) (err error) {
			c, err = d.Driver.Open(dsn)
			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		timeout,
	)
	return
}

func shouldRetry(err error) bool {
	underlying := err
	if op, ok := err.(*net.OpError); ok {
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
	if t, ok := err.(temporary); ok {
		return t.Temporary()
	}

	type timeout interface {
		Timeout() bool
	}
	if t, ok := err.(timeout); ok {
		return t.Timeout()
	}

	return false
}

func Register(logger *zap.SugaredLogger) {
	sql.Register("icingadb-mysql", &Driver{Driver: &mysql.MySQLDriver{}, Logger: logger})
	// TODO(el): Don't discard but hide?
	_ = mysql.SetLogger(log.New(ioutil.Discard, "", 0))
}
