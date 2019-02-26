package icingadb_connection

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var DbIoSeconds = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "db_io_seconds",
		Help: "Database I/O (s)",
	},
	[]string{"backend_type", "operation"},
)

var DbOperationsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_total",
	Help: "Database operations since startup",
})

var DbOperationsQuery = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_query",
	Help: "Database query operations since startup",
})

var DbOperationsExec = promauto.NewCounter(prometheus.CounterOpts{
	Name: "db_operations_exec",
	Help: "Database exec operations since startup",
})

//TODO: Move this to main package of IcingaDB
func Httpd(addr string, chErr chan error) {
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Serving debug info at http://%s/metrics", addr)
	chErr <- http.ListenAndServe(addr, nil)
}
