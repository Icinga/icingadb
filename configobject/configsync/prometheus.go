package configsync

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var ConfigSyncInsertsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "configsync_inserts_total",
		Help: "Config sync inserts total per object type",
	},
	[]string{"objecttype"},
)

var ConfigSyncDeletesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "configsync_deletes_total",
		Help: "Config sync deletes total per object type",
	},
	[]string{"objecttype"},
)

var ConfigSyncUpdatesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "configsync_updates_total",
		Help: "Config sync updates total per object type",
	},
	[]string{"objecttype"},
)
