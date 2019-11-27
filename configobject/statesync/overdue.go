// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package statesync

import (
	"encoding/hex"
	"fmt"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// luaGetOverdues takes the following KEYS:
// * either icinga:nextupdate:host or icinga:nextupdate:service
// * either icingadb:overdue:host or icingadb:overdue:service
// * a random one
//
// It takes the following ARGV:
// * the current date and time as *nix timestamp float in seconds
//
// It returns the following:
// * overdue monitored objects not yet marked overdue
// * not overdue monitored objects not yet unmarked overdue
var luaGetOverdues = redis.NewScript(`

local icingaNextupdate = KEYS[1]
local icingadbOverdue = KEYS[2]
local tempOverdue = KEYS[3]
local now = ARGV[1]

redis.call('DEL', tempOverdue)

local zrbs = redis.call('ZRANGEBYSCORE', icingaNextupdate, '-inf', '(' .. now)
for i = 1, #zrbs do
	redis.call('SADD', tempOverdue, zrbs[i])
end
zrbs = nil

local res = {redis.call('SDIFF', tempOverdue, icingadbOverdue), redis.call('SDIFF', icingadbOverdue, tempOverdue)}

redis.call('DEL', tempOverdue)

return res

`)

// overdueSyncCounters count on how many host/service is_overdue have synced since the last logOverdueSyncCounters().
var overdueSyncCounters = struct {
	host, service uint64
}{}

var updateHostOverdue = connection.DbIoSeconds.WithLabelValues("mysql", "update host_state by host_id")
var updateServiceOverdue = connection.DbIoSeconds.WithLabelValues("mysql", "update service_state by service_id")

// startOverdueSync starts the sync goroutines for hosts and services.
func startOverdueSync(super *supervisor.Supervisor) {
	go syncOverdue(super, "host", &overdueSyncCounters.host, updateHostOverdue)
	go syncOverdue(super, "service", &overdueSyncCounters.service, updateServiceOverdue)
	go logOverdueSyncCounters()
}

// logOverdueSyncCounters logs the amount of synced is_overdue every 20 seconds.
func logOverdueSyncCounters() {
	every20s := time.NewTicker(time.Second * 20)
	defer every20s.Stop()

	for {
		<-every20s.C

		host := atomic.SwapUint64(&overdueSyncCounters.host, 0)
		service := atomic.SwapUint64(&overdueSyncCounters.service, 0)

		if host > 0 || service > 0 {
			log.WithFields(log.Fields{
				"host":    host,
				"service": service,
				"period":  20 * time.Second,
			}).Infof("Synced some host and service overdue indicators")
		}
	}
}

// syncOverdue tries to sync is_overdue of given object type every second.
func syncOverdue(super *supervisor.Supervisor, objectType string, counter *uint64, observer prometheus.Observer) {
	for super.EnvId == nil {
		log.Debug("OverdueSync: Waiting for EnvId to be set")
		time.Sleep(time.Second)
	}

	every1s := time.NewTicker(time.Second)
	defer every1s.Stop()

	keys := [3]string{"icinga:nextupdate:" + objectType, "icingadb:overdue:" + objectType, ""}
	for {
		uuid4, errNR := uuid.NewRandom()
		if errNR != nil {
			log.WithFields(log.Fields{"error": errNR}).Error("Couldn't generate a new UUIDv4")
			time.Sleep(time.Second)
			continue
		}

		keys[2] = uuid4.String()
		break
	}

	for {
		overdues, errRS := luaGetOverdues.Run(
			super.Rdbw,
			keys[:],
			strconv.FormatFloat(utils.TimeToFloat(time.Now()), 'f', -1, 64),
		).Result()
		if errRS != nil {
			super.ChErr <- errRS
			time.Sleep(time.Second)
			continue
		}

		root := overdues.([]interface{})

		updateOverdue(super, objectType, counter, observer, root[0].([]interface{}), true)
		updateOverdue(super, objectType, counter, observer, root[1].([]interface{}), false)

		<-every1s.C
	}
}

// updateOverdue sets objectType_state#is_overdue for ids to overdue
// and updates icingadb:overdue:objectType respectively.
func updateOverdue(super *supervisor.Supervisor, objectType string, counter *uint64, observer prometheus.Observer, ids []interface{}, overdue bool) {
	if len(ids) > 0 {
		placeholders := make([]string, 0, len(ids))
		for len(placeholders) < cap(placeholders) {
			placeholders = append(placeholders, "?")
		}

		args := make([]interface{}, 0, len(ids))
		for _, hexId := range ids {
			id, errHD := hex.DecodeString(hexId.(string))
			if errHD != nil {
				super.ChErr <- errHD
				return
			}

			args = append(args, id)
		}

		_, errSE := super.Dbw.SqlExec(
			observer,
			fmt.Sprintf(
				"UPDATE %s_state SET is_overdue='%s' WHERE %s_id IN (%s)",
				objectType, utils.Bool[overdue], objectType, strings.Join(placeholders, ","),
			),
			args...,
		)
		if errSE != nil {
			super.ChErr <- errSE
			return
		}

		atomic.AddUint64(counter, uint64(len(ids)))

		var op func(key string, members ...interface{}) *redis.IntCmd
		if overdue {
			op = super.Rdbw.SAdd
		} else {
			op = super.Rdbw.SRem
		}

		if _, errOp := op("icingadb:overdue:"+objectType, ids...).Result(); errOp != nil {
			super.ChErr <- errSE
		}
	}
}
