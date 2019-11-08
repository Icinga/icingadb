// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package history

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"strconv"
	"sync"
	"time"
)

var mysqlObservers = func() (mysqlObservers map[string]prometheus.Observer) {
	mysqlObservers = map[string]prometheus.Observer{}

	for _, historyType := range [6]string{"state", "notification", "usernotification", "downtime", "comment", "flapping"} {
		mysqlObservers[historyType] = connection.DbIoSeconds.WithLabelValues(
			"mysql", fmt.Sprintf("replace into %s_history", historyType),
		)
	}

	return
}()

var historyCounter = make(map[string]int)
var historyCounterLock = sync.Mutex{}

var emptyUUID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
var emptyID = []byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

//Logs the amount of history entries added every 20 seconds
func logHistoryCounters() {
	every20s := time.NewTicker(time.Second * 20)
	defer every20s.Stop()

	for {
		<-every20s.C
		for _, historyType := range [6]string{"state", "notification", "usernotification", "downtime", "comment", "flapping"} {
			if historyCounter[historyType] > 0 {
				log.Infof("Added %d %s history entries in the last 20 seconds", historyCounter[historyType], historyType)
				historyCounterLock.Lock()
				historyCounter[historyType] = 0
				historyCounterLock.Unlock()
			}
		}
	}
}

func StartHistoryWorkers(super *supervisor.Supervisor) {
	workers := []func(supervisor2 *supervisor.Supervisor){
		notificationHistoryWorker,
		userNotificationHistoryWorker,
		stateHistoryWorker,
		downtimeHistoryWorker,
		commentHistoryWorker,
		flappingHistoryWorker,
	}

	for workerId := range workers {
		worker := workers[workerId]
		go func() {
			for {
				worker(super)
			}
		}()
	}

	go logHistoryCounters()
}

func notificationHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO notification_history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_id, type,` +
			"event_time, state, previous_hard_state, author, `text`, users_notified)" +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				id[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				utils.EncodeChecksum(values["notification_id"].(string)),
				utils.NotificationTypesToDbEnumString[values["type"].(string)],
				values["event_time"],
				values["state"],
				values["previous_hard_state"],
				values["author"],
				values["text"],
				values["users_notified"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			notificationHistoryId := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				notificationHistoryId[:],
				emptyUUID[:],
				emptyID,
				emptyID,
				emptyUUID[:],
				values["event_type"],
				values["event_time"],
			}

			return data
		},
	}

	historyWorker(super, "notification", statements, dataFunctions, mysqlObservers["notification"])
}

func userNotificationHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO user_notification_history (id, environment_id, notification_history_id, user_id)` +
			`VALUES (?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			notificationHistoryId := uuid.MustParse(values["notification_history_id"].(string))
			data := []interface{}{
				id[:],
				super.EnvId,
				notificationHistoryId[:],
				utils.EncodeChecksum(values["user_id"].(string)),
			}

			return data
		},
	}

	historyWorker(super, "usernotification", statements, dataFunctions, mysqlObservers["usernotification"])
}

func stateHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO state_history (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time, state_type,` +
			`soft_state, hard_state, previous_soft_state, previous_hard_state, attempt, last_soft_state, last_hard_state, output, long_output, max_check_attempts, check_source)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			stateType, err := strconv.ParseFloat(values["state_type"].(string), 32)

			if err != nil {
				log.Errorf("StateHistory: Could not parse stateType (%s) into float32", values["state_type"])
			}

			data := []interface{}{
				id[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["event_time"],
				utils.IcingaStateTypeToString(float32(stateType)),
				values["soft_state"],
				values["hard_state"],
				values["previous_soft_state"],
				values["previous_hard_state"],
				values["attempt"],
				values["last_soft_state"],
				values["last_hard_state"],
				values["output"],
				values["long_output"],
				values["max_check_attempts"],
				values["check_source"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			stateHistoryId := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				emptyUUID[:],
				stateHistoryId[:],
				emptyID,
				emptyID,
				emptyUUID[:],
				values["event_type"],
				values["event_time"],
			}

			return data
		},
	}

	historyWorker(super, "state", statements, dataFunctions, mysqlObservers["state"])
}

func downtimeHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO downtime_history (downtime_id, environment_id, endpoint_id, triggered_by_id, object_type, host_id, service_id, entry_time,` +
			`author, comment, is_flexible, flexible_duration, scheduled_start_time, scheduled_end_time, start_time, end_time, has_been_cancelled, trigger_time, cancel_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			var triggeredById []byte
			if values["triggered_by_id"] != nil {
				triggeredById = utils.EncodeChecksum(values["triggered_by_id"].(string))
			}

			data := []interface{}{
				utils.EncodeChecksum(values["downtime_id"].(string)),
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				triggeredById,
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["entry_time"],
				values["author"],
				values["comment"],
				utils.RedisIntToDBBoolean(values["is_flexible"]),
				values["flexible_duration"],
				values["scheduled_start_time"],
				values["scheduled_end_time"],
				values["start_time"],
				values["end_time"],
				utils.RedisIntToDBBoolean(values["has_been_cancelled"]),
				values["trigger_time"],
				values["cancel_time"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			downtimeHistoryId := utils.EncodeChecksum(values["downtime_id"].(string))

			var eventTime string
			switch values["event_type"] {
			case "downtime_start":
				eventTime = values["start_time"].(string)
			case "downtime_end":
				eventTime = values["end_time"].(string)
			}

			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				emptyUUID[:],
				emptyUUID[:],
				downtimeHistoryId,
				emptyID,
				emptyUUID[:],
				values["event_type"],
				eventTime,
			}

			return data
		},
	}

	historyWorker(super, "downtime", statements, dataFunctions, mysqlObservers["downtime"])
}

func commentHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO comment_history (comment_id, environment_id, endpoint_id, object_type, host_id, service_id, entry_time, author,` +
			`comment, entry_type, is_persistent, is_sticky, expire_time, remove_time, has_been_removed)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			data := []interface{}{
				utils.EncodeChecksum(values["comment_id"].(string)),
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["entry_time"],
				values["author"],
				values["comment"],
				utils.CommentEntryTypes[values["entry_type"].(string)],
				utils.RedisIntToDBBoolean(values["is_persistent"]),
				utils.RedisIntToDBBoolean(values["is_sticky"]),
				values["expire_time"],
				values["remove_time"],
				utils.RedisIntToDBBoolean(values["has_been_removed"]),
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			commentHistoryId := utils.EncodeChecksum(values["comment_id"].(string))

			var eventTime string
			switch values["event_type"] {
			case "comment_add":
				eventTime = values["entry_time"].(string)
			case "comment_remove":
				if values["remove_time"] != nil {
					eventTime = values["remove_time"].(string)
				} else {
					eventTime = values["expire_time"].(string)
				}
			}

			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				emptyUUID[:],
				emptyUUID[:],
				emptyID,
				commentHistoryId,
				emptyUUID[:],
				values["event_type"],
				eventTime,
			}

			return data
		},
	}

	historyWorker(super, "comment", statements, dataFunctions, mysqlObservers["comment"])
}

func flappingHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO flapping_history (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time, event_type,` +
			`percent_state_change, flapping_threshold_low, flapping_threshold_high)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				id[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["event_time"],
				values["event_type"],
				values["percent_state_change"],
				values["flapping_threshold_low"],
				values["flapping_threshold_high"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			flappingHistoryId := uuid.MustParse(values["id"].(string))

			var eventTime string
			switch values["event_type"] {
			case "flapping_start":
				eventTime = values["start_time"].(string)
			case "flapping_end":
				eventTime = values["end_time"].(string)
			}

			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.DecodeHexIfNotNil(values["host_id"]),
				utils.DecodeHexIfNotNil(values["service_id"]),
				emptyUUID[:],
				emptyUUID[:],
				emptyID,
				emptyID,
				flappingHistoryId[:],
				values["event_type"],
				eventTime,
			}

			return data
		},
	}

	historyWorker(super, "flapping", statements, dataFunctions, mysqlObservers["flapping"])
}

func historyWorker(super *supervisor.Supervisor, historyType string, preparedStatements []string, dataFunctions []func(map[string]interface{}) []interface{}, observer prometheus.Observer) {
	if super.EnvId == nil {
		log.Debug(historyType + "History: Waiting for EnvId to be set")
		<-time.NewTimer(time.Second).C
		return
	}

	result := super.Rdbw.XRead(&redis.XReadArgs{Block: 0, Count: 1000, Streams: []string{"icinga:history:stream:" + historyType, "0"}})
	streams, err := result.Result()
	if err != nil {
		super.ChErr <- err
		return
	}

	entries := streams[0].Messages
	if len(entries) == 0 {
		return
	}

	log.Debugf("%d %s history entries will be synced", len(entries), historyType)
	var storedEntryIds []string
	brokenEntries := 0

	for _, state := range entries {
		storedEntryIds = append(storedEntryIds, state.ID)
	}

	for {
		errTx := super.Dbw.SqlTransaction(false, true, false, func(tx connection.DbTransaction) error {
			for i, state := range entries {
				for statementIndex, statement := range preparedStatements {
					_, errExec := super.Dbw.SqlExecTx(
						tx,
						observer,
						statement,
						dataFunctions[statementIndex](state.Values)...,
					)

					if errExec != nil {
						log.WithFields(log.Fields{
							"context": historyType + "History",
							"values":  state.Values,
						}).Error(errExec)

						entries = removeEntryFromEntriesSlice(entries, i)

						brokenEntries++

						return errExec
					}
				}
			}

			return nil
		})

		if errTx != nil {
			log.WithFields(log.Fields{
				"context": historyType + "History",
			}).Error(errTx)
		} else {
			break
		}
	}

	//Delete synced entries from redis stream
	super.Rdbw.XDel("icinga:history:stream:"+historyType, storedEntryIds...)

	count := len(storedEntryIds) - brokenEntries

	historyCounterLock.Lock()
	historyCounter[historyType]++
	historyCounterLock.Unlock()

	log.Debugf("%d %s history entries synced", count, historyType)
	log.Debugf("%d %s history entries broken", brokenEntries, historyType)
}

//Removes one redis.XMessage at given index from given slice and returns the resulting slice
func removeEntryFromEntriesSlice(s []redis.XMessage, i int) []redis.XMessage {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}
