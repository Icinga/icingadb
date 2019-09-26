package statesync

import (
	"encoding/hex"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"time"
)

//Counter on how many host/service states have synced since the last logSyncCounters()
var syncCounter = make(map[string]int)

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
		every1s := time.NewTicker(time.Second)

		for {
			<-every1s.C
			syncStates(super,"host")
		}
	}()

	go func() {
		every1s := time.NewTicker(time.Second)

		for {
			<-every1s.C
			syncStates(super,"service")
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
		log.Infof("Synced %d host and %d service states in the last 20 seconds", syncCounter["host"], syncCounter["service"])
		syncCounter = make(map[string]int)
	}
}

//Tries to sync the states of given object type every second
func syncStates(super *supervisor.Supervisor, objectType string) {
	if super.EnvId == nil {
		log.Debug("StateSync: Waiting for EnvId to be set")
		return
	}

	result := super.Rdbw.XRead(&redis.XReadArgs{Streams: []string{"icinga:state:stream:" + objectType, "0"}})
	streams, err := result.Result()
	if err != nil {
		super.ChErr <- err
		return
	}

	states := streams[0].Messages
	if len(states) == 0 {
		return
	}

	errTx := super.Dbw.SqlTransaction(true, true, false, func(tx connection.DbTransaction) error {
		for _, state := range states {
			values := state.Values

			id, _ := hex.DecodeString(values["id"].(string))

			var acknowledgementCommentId []byte
			if values["acknowledgement_comment_id"] != nil {
				acknowledgementCommentId, _ = hex.DecodeString(values["acknowledgement_comment_id"].(string))
			}

			_, errExec := super.Dbw.SqlExecTx(
				tx,
				mysqlObservers[objectType],
				`REPLACE INTO `+objectType+`_state (`+objectType+`_id, environment_id, state_type, soft_state, hard_state, attempt, severity, output, long_output, performance_data,`+
					`check_commandline, is_problem, is_handled, is_reachable, is_flapping, is_acknowledged, acknowledgement_comment_id,`+
					`in_downtime, execution_time, latency, timeout, last_update, last_state_change, last_soft_state,`+
					`last_hard_state, next_check, next_update) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				id,
				super.EnvId,
				redisStateTypeToDBStateType(values["state_type"]),
				values["state"],
				values["last_hard_state"],
				values["check_attempt"],
				redisIntToDBInt(values["severity"]),
				values["output"],
				values["long_output"],
				values["performance_data"],
				values["commandline"],
				redisBooleanToDBBoolean(values["is_problem"]),
				redisBooleanToDBBoolean(values["is_handled"]),
				redisBooleanToDBBoolean(values["is_reachable"]),
				redisBooleanToDBBoolean(values["is_flapping"]),
				redisBooleanToDBBoolean(values["is_acknowledged"]),
				acknowledgementCommentId,
				redisBooleanToDBBoolean(values["in_downtime"]),
				values["execution_time"],
				redisIntToDBInt(values["latency"]),
				redisIntToDBInt(values["check_timeout"]),
				values["last_update"],
				values["last_state_change"],
				values["last_soft_state"],
				values["last_hard_state"],
				values["next_check"],
				values["next_update"],
			)

			if errExec != nil {
				return errExec
			}
		}

		return nil
	})

	if errTx != nil {
		super.ChErr <- errTx
		return
	}

	//Delete synced states from redis stream
	var storedStateIds []string
	for _, state := range states {
		storedStateIds = append(storedStateIds, state.ID)
	}

	super.Rdbw.XDel("icinga:state:stream:"+objectType, storedStateIds...)

	log.Debugf("%d %s state synced", len(storedStateIds), objectType)
	syncCounter[objectType] += len(storedStateIds)
	StateSyncsTotal.WithLabelValues(objectType).Add(float64(len(storedStateIds)))
}

//Converts a Icinga state type(0 for soft, 1 for hard) we got from Redis into a DB state type(soft, hard)
func redisStateTypeToDBStateType(value interface{}) string {
	if value == "1" {
		return "hard"
	} else {
		return "soft"
	}
}

//Converts a boolean we got from Redis into a DB boolean
func redisBooleanToDBBoolean(value interface{}) string {
	if value == "true" {
		return "y"
	} else { //Should catch empty strings and nil
		return "n"
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
