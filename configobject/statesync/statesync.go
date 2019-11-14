// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package statesync

import (
	"encoding/hex"
	"github.com/go-redis/redis"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

//Counter on how many host/service states have synced since the last logSyncCounters()
var syncCounter = make(map[string]int)
var syncCounterLock = sync.Mutex{}

var mysqlObservers = func() (mysqlObservers map[string]prometheus.Observer) {
	mysqlObservers = map[string]prometheus.Observer{}

	for _, objectType := range [2]string{"host", "service"} {
		mysqlObservers[objectType] = connection.DbIoSeconds.WithLabelValues("mysql", "replace into "+objectType+"_state")
	}

	return
}()

//Start the sync goroutines for hosts and services
func StartStateSync(super *supervisor.Supervisor) {
	go func() {
		for {
			syncStates(super, "host")
		}
	}()

	go func() {
		for {
			syncStates(super, "service")
		}
	}()

	go logSyncCounters()
}

//Logs the amount of synced states every 20 seconds
func logSyncCounters() {
	every20s := time.NewTicker(time.Second * 20)
	defer every20s.Stop()

	for {
		<-every20s.C
		if syncCounter["host"] > 0 || syncCounter["service"] > 0 {
			log.Infof("Synced %d host and %d service states in the last 20 seconds", syncCounter["host"], syncCounter["service"])
			syncCounterLock.Lock()
			syncCounter = make(map[string]int)
			syncCounterLock.Unlock()
		}
	}
}

//Tries to sync the states of given object type every second
func syncStates(super *supervisor.Supervisor, objectType string) {
	if super.EnvId == nil {
		log.Debug("StateSync: Waiting for EnvId to be set")
		<-time.NewTimer(time.Second).C
		return
	}

	result := super.Rdbw.XRead(&redis.XReadArgs{Block: 0, Count: 1000, Streams: []string{"icinga:state:stream:" + objectType, "0"}})
	streams, err := result.Result()
	if err != nil {
		super.ChErr <- err
		return
	}

	states := streams[0].Messages
	if len(states) == 0 {
		return
	}

	log.Debugf("%d %s state will be synced", len(states), objectType)
	var storedStateIds []string
	brokenStates := 0

	for _, state := range states {
		storedStateIds = append(storedStateIds, state.ID)
	}

	for {
		errTx := super.Dbw.SqlTransaction(false, true, false, func(tx connection.DbTransaction) error {
			for i, state := range states {
				values := state.Values
				id, _ := hex.DecodeString(values["id"].(string))

				var acknowledgementCommentId []byte
				if values["acknowledgement_comment_id"] != nil {
					acknowledgementCommentId, _ = hex.DecodeString(values["acknowledgement_comment_id"].(string))
				}

				_, errExec := super.Dbw.SqlExecTx(
					tx,
					mysqlObservers[objectType],
					`REPLACE INTO `+objectType+`_state (`+objectType+`_id, environment_id, state_type, soft_state, hard_state, previous_hard_state, attempt, severity, output, long_output, performance_data,`+
						`check_commandline, is_problem, is_handled, is_reachable, is_flapping, is_acknowledged, acknowledgement_comment_id,`+
						`in_downtime, execution_time, latency, timeout, check_source, last_update, last_state_change, next_check, next_update) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
					id,
					super.EnvId,
					redisStateTypeToDBStateType(values["state_type"]),
					values["state"],
					values["hard_state"],
					values["previous_hard_state"],
					values["check_attempt"],
					redisIntToDBInt(values["severity"]),
					values["output"],
					values["long_output"],
					values["performance_data"],
					values["commandline"],
					utils.JSONBooleanToDBBoolean(values["is_problem"]),
					utils.JSONBooleanToDBBoolean(values["is_handled"]),
					utils.JSONBooleanToDBBoolean(values["is_reachable"]),
					utils.JSONBooleanToDBBoolean(values["is_flapping"]),
					utils.JSONBooleanToDBBoolean(values["is_acknowledged"]),
					acknowledgementCommentId,
					utils.JSONBooleanToDBBoolean(values["in_downtime"]),
					values["execution_time"],
					redisIntToDBInt(values["latency"]),
					redisIntToDBInt(values["check_timeout"]),
					values["check_source"],
					values["last_update"],
					values["last_state_change"],
					values["next_check"],
					values["next_update"],
				)

				if errExec != nil {
					log.WithFields(log.Fields{
						"context":    "StateSync",
						"objectType": objectType,
						"state":      values,
					}).Error(errExec)

					states = removeStateFromStatesSlice(states, i)

					brokenStates++

					return errExec
				}
			}

			return nil
		})

		if errTx != nil {
			log.WithFields(log.Fields{
				"context": "StateSync",
			}).Error(errTx)
		} else {
			break
		}
	}

	//Delete synced states from redis stream
	super.Rdbw.XDel("icinga:state:stream:"+objectType, storedStateIds...)

	log.Debugf("%d %s state synced", len(storedStateIds)-brokenStates, objectType)
	log.Debugf("%d %s state broken", brokenStates, objectType)
	syncCounterLock.Lock()
	syncCounter[objectType] += len(storedStateIds)
	syncCounterLock.Unlock()
	StateSyncsTotal.WithLabelValues(objectType).Add(float64(len(storedStateIds)))
}

//Removes one redis.XMessage at given index from given slice and returns the resulting slice
func removeStateFromStatesSlice(s []redis.XMessage, i int) []redis.XMessage {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

//Converts a Icinga state type(0 for soft, 1 for hard) we got from Redis into a DB state type(soft, hard)
func redisStateTypeToDBStateType(value interface{}) string {
	if value == "1" {
		return "hard"
	} else {
		return "soft"
	}
}

//Converts a int we got from Redis into a DB int
func redisIntToDBInt(value interface{}) string {
	if value == nil || value == "" {
		return "0"
	} else {
		return value.(string)
	}
}
