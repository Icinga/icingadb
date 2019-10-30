package connection

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var DbIoSeconds = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "db_io_seconds",
		Help: "Database I/O (s)",
	},
	[]string{"backend_type", "operation"},
)

var DbOperationsQuery = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_query",
	Help: "Database query operations since startup",
})

var DbOperationsExec = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_exec",
	Help: "Database exec operations since startup",
})

var DbOperationsBegin = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_begin",
	Help: "Database begin operations since startup",
})

var DbOperationsCommit = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_commit",
	Help: "Database commit operations since startup",
})

var DbOperationsRollback = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_rollback",
	Help: "Database rollback operations since startup",
})

var DbTransactions = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_transactions",
	Help: "Database transactions since startup",
})

var DbFetchIds = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_fetch_ids",
	Help: "Database FetchId calls since startup",
})

var DbFetchChecksums = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_fetch_checksums",
	Help: "Database FetchChecksums calls since startup",
})

var DbBulkInserts = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_bulk_inserts",
	Help: "Database bulk inserts since startup",
})

var DbBulkUpdates = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_bulk_updates",
	Help: "Database bulk updates since startup",
})

var DbBulkDeletes = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_bulk_deletes",
	Help: "Database bulk deletes since startup",
})
