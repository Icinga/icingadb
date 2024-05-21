package database

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/strcase"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/pkg/errors"
)

// CantPerformQuery wraps the given error with the specified query that cannot be executed.
func CantPerformQuery(err error, q string) error {
	return errors.Wrapf(err, "can't perform %q", q)
}

// TableName returns the table of t.
func TableName(t interface{}) string {
	if tn, ok := t.(TableNamer); ok {
		return tn.TableName()
	} else {
		return strcase.Snake(types.Name(t))
	}
}

// SplitOnDupId returns a state machine which tracks the inputs' IDs.
// Once an already seen input arrives, it demands splitting.
func SplitOnDupId[T IDer]() com.BulkChunkSplitPolicy[T] {
	seenIds := map[string]struct{}{}

	return func(ider T) bool {
		id := ider.ID().String()

		_, ok := seenIds[id]
		if ok {
			seenIds = map[string]struct{}{id: {}}
		} else {
			seenIds[id] = struct{}{}
		}

		return ok
	}
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

var (
	_ com.BulkChunkSplitPolicyFactory[Entity] = SplitOnDupId[Entity]
)
