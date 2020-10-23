// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package statesync

import (
	"encoding/hex"
	"fmt"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/ha"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis/v7"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
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
func startOverdueSync(super *supervisor.Supervisor, haInstance *ha.HA) {
	go syncOverdue(super, haInstance.RegisterStateChangeListener(), "host", &overdueSyncCounters.host, updateHostOverdue)
	go syncOverdue(super, haInstance.RegisterStateChangeListener(), "service", &overdueSyncCounters.service, updateServiceOverdue)
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
func syncOverdue(super *supervisor.Supervisor, chHA <-chan ha.State, objectType string, counter *uint64, observer prometheus.Observer) {
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
		log.Infof("Waiting for HA to become active before (re)starting %s overdue sync", objectType)
		for <-chHA != ha.StateActive {
			// do nothing and wait to become active
		}
		log.Infof("HA became active, starting %s overdue sync", objectType)

		for {
			if err := syncOverdueToRedis(super, objectType, observer); err != nil {
				log.WithFields(log.Fields{"error": err}).Errorf("Couldn't fetch overdue %ss from database", objectType)
				time.Sleep(time.Second)
				continue
			}

			break
		}

	syncLoop:
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

			select {
			case <-every1s.C:
				// continue syncing
			case state := <-chHA:
				if state != ha.StateActive {
					log.Infof("HA became inactive, stopping %s overdue sync", objectType)
					break syncLoop
				}
			}
		}
	}
}

// updateOverdue sets objectType_state#is_overdue for ids to overdue
// and updates icingadb:overdue:objectType respectively.
func updateOverdue(super *supervisor.Supervisor, objectType string, counter *uint64, observer prometheus.Observer, ids []interface{}, overdue bool) {
	if len(ids) > 0 {
		if updateOverdueInDb(super, objectType, observer, ids, overdue) {
			atomic.AddUint64(counter, uint64(len(ids)))

			if overdue {
				if _, errOp := super.Rdbw.SAdd("icingadb:overdue:"+objectType, ids...).Result(); errOp != nil {
					super.ChErr <- errOp
				}
			} else {
				if _, errOp := super.Rdbw.SRem("icingadb:overdue:"+objectType, ids...).Result(); errOp != nil {
					super.ChErr <- errOp
				}
			}
		}
	}
}

// updateOverdueInDb sets objectType_state#is_overdue for ids to overdue.
func updateOverdueInDb(super *supervisor.Supervisor, objectType string, observer prometheus.Observer, ids []interface{}, overdue bool) bool {
	group := errgroup.Group{}
	for _, c := range utils.ChunkInterfaces(ids, 1000) {
		chunk := c
		group.Go(func() error {
			placeholders := make([]string, 0, len(chunk))
			for len(placeholders) < cap(placeholders) {
				placeholders = append(placeholders, "?")
			}

			args := make([]interface{}, 0, len(chunk))
			for _, hexId := range chunk {
				id, errHD := hex.DecodeString(hexId.(string))
				if errHD != nil {
					return errHD
				}

				args = append(args, id)
			}

			log.WithFields(log.Fields{
				"amount":  len(chunk),
				"type":    objectType,
				"overdue": overdue,
			}).Debug("Syncing overdue indicators to database")

			_, errSE := super.Dbw.SqlExec(
				observer,
				fmt.Sprintf(
					"UPDATE %s_state SET is_overdue='%s' WHERE %s_id IN (%s)",
					objectType, utils.Bool[overdue], objectType, strings.Join(placeholders, ","),
				),
				args...,
			)

			return errSE
		})
	}

	err := group.Wait()
	if err != nil {
		super.ChErr <- err
		return false
	}

	return true
}

func syncOverdueToRedis(super *supervisor.Supervisor, objectType string, observer prometheus.Observer) (error) {
	type row struct {
		Id []byte
	}

	overdueRows, err := super.Dbw.SqlFetchAll(observer, row{},
		fmt.Sprintf("SELECT %s_id FROM %s_state WHERE is_overdue = ?", objectType, objectType),
		utils.Bool[true])
	if err != nil {
		return err
	}

	if _, err := super.Rdbw.Del("icingadb:overdue:"+objectType).Result(); err != nil {
		return err
	}

	for _, row := range overdueRows.([]row) {
		if _, err := super.Rdbw.SAdd("icingadb:overdue:"+objectType, hex.EncodeToString(row.Id)).Result(); err != nil {
			return err
		}
	}

	return nil
}
