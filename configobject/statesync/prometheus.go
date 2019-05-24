package statesync

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var StateSyncsPerSecond = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "statesyncs_per_second",
		Help: "Statesyncs per second per object type",
	},
	[]string{"objecttype"},
)
