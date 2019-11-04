// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package prometheus

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func HandleHttp(addr string, chErr chan error) {
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Serving metrics at http://%s/metrics", addr)
	chErr <- http.ListenAndServe(addr, nil)
}
