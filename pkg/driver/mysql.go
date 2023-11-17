package driver

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

var errUnknownSysVar = &mysql.MySQLError{Number: 1193}

// setGaleraOpts tries SET SESSION wsrep_sync_wait=4.
//
// This ensures causality checks will take place before executing INSERT or REPLACE,
// ensuring that the statement is executed on a fully synced node.
// https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_sync_wait
//
// It prevents running into foreign key errors while inserting into linked tables on different MySQL nodes.
// Error 1193 "Unknown system variable" is ignored to support MySQL single nodes.
func setGaleraOpts(ctx context.Context, conn driver.Conn) error {
	const galeraOpts = "SET SESSION wsrep_sync_wait=4"

	stmt, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, galeraOpts)
	if err != nil {
		err = errors.Wrap(err, "can't prepare "+galeraOpts)
	} else if _, err = stmt.(driver.StmtExecContext).ExecContext(ctx, nil); err != nil {
		err = errors.Wrap(err, "can't execute "+galeraOpts)
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
