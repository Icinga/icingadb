// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package statesync

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var StateSyncsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "statesyncs_total",
		Help: "Statesyncs total per object type",
	},
	[]string{"objecttype"},
)
