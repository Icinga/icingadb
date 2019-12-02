// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package prometheus

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func HandleHttp(addr string, chErr chan error) {
	http.Handle("/metrics", promhttp.Handler())
	log.WithFields(log.Fields{"address": fmt.Sprintf("http://%s/metrics", addr)}).Info("Serving metrics")
	chErr <- http.ListenAndServe(addr, nil)
}
