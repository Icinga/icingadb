package driver

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"strconv"
)

// MySQLDriver extends mysql.MySQLDriver with auto-SETting Galera cluster options.
type MySQLDriver struct {
	mysql.MySQLDriver
}

// Open implements the driver.Driver interface.
func (md MySQLDriver) Open(name string) (driver.Conn, error) {
	connector, err := md.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return connector.Connect(context.Background())
}

// OpenConnector implements the driver.DriverContext interface.
func (md MySQLDriver) OpenConnector(name string) (driver.Connector, error) {
	var wsrepSyncWait int64

	if config, err := mysql.ParseDSN(name); err == nil {
		if s, ok := config.Params["wsrep_sync_wait"]; ok {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				// MySQL single nodes don't know wsrep_sync_wait and fail with error 1193 "Unknown system variable".
				// We have to SET it manually later and swallow error 1193 not to fail our connections.
				wsrepSyncWait = i
				delete(config.Params, "wsrep_sync_wait")
				name = config.FormatDSN()
			}
		}
	}

	connector, err := md.MySQLDriver.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return &galeraAwareConnector{connector, wsrepSyncWait, md}, nil
}

// galeraAwareConnector extends mysql.connector with auto-SETting Galera cluster options.
type galeraAwareConnector struct {
	driver.Connector

	wsrepSyncWait int64
	driver        driver.Driver
}

// Connect implements the driver.Connector interface.
func (gac *galeraAwareConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := gac.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	if err := setGaleraOpts(ctx, conn, gac.wsrepSyncWait); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

// Driver implements the driver.Connector interface.
func (gac *galeraAwareConnector) Driver() driver.Driver {
	return gac.driver
}

var errUnknownSysVar = &mysql.MySQLError{Number: 1193}

// setGaleraOpts tries SET SESSION wsrep_sync_wait.
//
// This ensures causality checks will take place before execution,
// ensuring that every statement is executed on a fully synced node.
// https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_sync_wait
//
// It prevents running into foreign key errors while inserting into linked tables on different MySQL nodes.
// Error 1193 "Unknown system variable" is ignored to support MySQL single nodes.
func setGaleraOpts(ctx context.Context, conn driver.Conn, wsrepSyncWait int64) error {
	const galeraOpts = "SET SESSION wsrep_sync_wait=?"

	stmt, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, galeraOpts)
	if err != nil {
		err = errors.Wrap(err, "can't prepare "+galeraOpts)
	} else {
		_, err = stmt.(driver.StmtExecContext).ExecContext(ctx, []driver.NamedValue{{Value: wsrepSyncWait}})
		if err != nil {
			err = errors.Wrap(err, "can't execute "+galeraOpts)
		}
	}

	if err != nil && errors.Is(err, errUnknownSysVar) {
		err = nil
	}

	if stmt != nil {
		if errClose := stmt.Close(); errClose != nil && err == nil {
			err = errors.Wrap(errClose, "can't close statement "+galeraOpts)
		}
	}

	return err
}

// Assert interface compliance.
var (
	_ driver.Driver        = MySQLDriver{}
	_ driver.DriverContext = MySQLDriver{}
	_ driver.Connector     = (*galeraAwareConnector)(nil)
)
