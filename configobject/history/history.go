package history

import (
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"git.icinga.com/icingadb/icingadb-main/utils"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

var mysqlObservers = func() (mysqlObservers map[string]map[string]prometheus.Observer) {
	mysqlObservers = map[string]map[string]prometheus.Observer{}

	for _, historyType := range [5]string{"state", "notification", "downtime", "comment", "flapping"} {
		mysqlObservers[historyType] = map[string]prometheus.Observer{}
		for _, objectType := range [2]string{"host", "service"} {
			mysqlObservers[historyType][objectType] = connection.DbIoSeconds.WithLabelValues("mysql", "replace into " + objectType + "_" + historyType + "_history")
		}
	}

	return
}()

var emptyUUID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
var emptyID = []byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

func StartHistoryWorkers(super *supervisor.Supervisor) {
	workers := []func(supervisor2 *supervisor.Supervisor, objectType string){
		notificationHistoryWorker,
		stateHistoryWorker,
		downtimeHistoryWorker,
		commentHistoryWorker,
		flappingHistoryWorker,
	}

	for workerId := range workers {
		worker := workers[workerId]
		go func() {
			for {
				worker(super, "host")
			}
		}()
		go func() {
			for {
				worker(super, "service")
			}
		}()
	}
}

func notificationHistoryWorker(super *supervisor.Supervisor, objectType string) {
	statements := []string{
		`REPLACE INTO `+objectType+`_notification_history (id, environment_id, `+objectType+`_id, notification_id, type,` +
			`send_time, state, output, long_output, users_notified)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO `+objectType+`_history (id, environment_id, `+objectType+`_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				id[:],
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				utils.EncodeChecksum(values["notification_id"].(string)),
				values["type"],
				values["send_time"],
				values["state"],
				values["output"],
				values["long_output"],
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
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				notificationHistoryId[:],
				emptyUUID[:],
				emptyID,
				emptyID,
				emptyUUID[:],
				values["event_type"],
				values["send_time"],
			}

			return data
		},
	}

	historyWorker(super, objectType, "notification", statements, dataFunctions, mysqlObservers["notification"][objectType])
}

func stateHistoryWorker(super *supervisor.Supervisor, objectType string) {
	statements := []string{
		`REPLACE INTO `+objectType+`_state_history (id, environment_id, `+objectType+`_id, change_time, state_type,` +
			`soft_state, hard_state, attempt, last_soft_state, last_hard_state, output, long_output, max_check_attempts)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO `+objectType+`_history (id, environment_id, `+objectType+`_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
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
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				values["change_time"],
				utils.IcingaStateTypeToString(float32(stateType)),
				values["soft_state"],
				values["hard_state"],
				values["attempt"],
				values["last_soft_state"],
				values["last_hard_state"],
				values["output"],
				values["long_output"],
				values["max_check_attempts"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			stateHistoryId := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				emptyUUID[:],
				stateHistoryId[:],
				emptyID,
				emptyID,
				emptyUUID[:],
				values["event_type"],
				values["change_time"],
			}

			return data
		},
	}

	historyWorker(super, objectType, "state", statements, dataFunctions, mysqlObservers["state"][objectType])
}

func downtimeHistoryWorker(super *supervisor.Supervisor, objectType string) {
	statements := []string{
		`REPLACE INTO ` + objectType + `_downtime_history (downtime_id, environment_id, ` + objectType + `_id, triggered_by_id, entry_time,` +
			`author, comment, is_flexible, flexible_duration, scheduled_start_time, scheduled_end_time, was_started, actual_start_time, actual_end_time, was_cancelled, is_in_effect, trigger_time, deletion_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO `+objectType+`_history (id, environment_id, `+objectType+`_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{}{
			var triggeredById []byte
			if values["triggered_by_id"] != nil{
				triggeredById = utils.EncodeChecksum(values["triggered_by_id"].(string))
			}

			data := []interface{}{
				utils.EncodeChecksum(values["downtime_id"].(string)),
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				triggeredById,
				values["entry_time"],
				values["author"],
				values["comment"],
				utils.RedisIntToDBBoolean(values["is_flexible"]),
				values["flexible_duration"],
				values["scheduled_start_time"],
				values["scheduled_end_time"],
				utils.RedisIntToDBBoolean(values["was_started"]),
				values["actual_start_time"],
				values["actual_end_time"],
				utils.RedisIntToDBBoolean(values["was_cancelled"]),
				utils.RedisIntToDBBoolean(values["is_in_effect"]),
				values["trigger_time"],
				values["deletion_time"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))
			downtimeHistoryId := utils.EncodeChecksum(values["downtime_id"].(string))

			var eventTime string
			switch values["event_type"] {
			case "downtime_schedule":
				eventTime = values["entry_time"].(string)
			case "downtime_start":
				eventTime = values["actual_start_time"].(string)
			case "downtime_end":
				eventTime = values["actual_end_time"].(string)
			}

			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
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

	historyWorker(super, objectType, "downtime", statements, dataFunctions, mysqlObservers["downtime"][objectType])
}

func commentHistoryWorker(super *supervisor.Supervisor, objectType string) {
	statements := []string{
		`REPLACE INTO `+objectType+`_comment_history (comment_id, environment_id, `+objectType+`_id, entry_time, author,` +
			`comment, entry_type, is_persistent, expire_time, deletion_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO `+objectType+`_history (id, environment_id, `+objectType+`_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			data := []interface{}{
				utils.EncodeChecksum(values["comment_id"].(string)),
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				values["entry_time"],
				values["author"],
				values["comment"],
				utils.CommentEntryTypes[values["entry_type"].(string)],
				utils.RedisIntToDBBoolean(values["is_persistent"]),
				values["expire_time"],
				values["deletion_time"],
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
				eventTime = values["deletion_time"].(string)
			}

			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
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

	historyWorker(super, objectType, "comment", statements, dataFunctions, mysqlObservers["comment"][objectType])
}

func flappingHistoryWorker(super *supervisor.Supervisor, objectType string) {
	statements := []string{
		`REPLACE INTO `+objectType+`_flapping_history (id, environment_id, `+objectType+`_id, start_time, end_time,` +
			`percent_state_change, flapping_threshold_low, flapping_threshold_high)` +
			`VALUES (?,?,?,?,?,?,?,?)`,
		`REPLACE INTO `+objectType+`_history (id, environment_id, `+objectType+`_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				id[:],
				super.EnvId,
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
				values["start_time"],
				values["end_time"],
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
				utils.EncodeChecksum(values[objectType+"_id"].(string)),
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

	historyWorker(super, objectType, "flapping", statements, dataFunctions, mysqlObservers["flapping"][objectType])
}

func historyWorker(super *supervisor.Supervisor, objectType string, historyType string, preparedStatements []string, dataFunctions []func(map[string]interface{}) []interface{}, observer prometheus.Observer) {
	if super.EnvId == nil {
		log.Debug( historyType + "History: Waiting for EnvId to be set")
		<- time.NewTimer(time.Second).C
		return
	}

	result := super.Rdbw.XRead(&redis.XReadArgs{Block: 0, Count: 1000, Streams: []string{"icinga:history:stream:" + objectType + ":" + historyType, "0"}})
	streams, err := result.Result()
	if err != nil {
		super.ChErr <- err
		return
	}

	entries := streams[0].Messages
	if len(entries) == 0 {
		return
	}

	log.Debugf("%d %s %s history entries will be synced", len(entries), objectType, historyType)
	var storedEntryIds []string
	brokenEntries := 0

	for _, state := range entries {
		storedEntryIds = append(storedEntryIds, state.ID)
	}

	for {
		errTx := super.Dbw.SqlTransaction(true, true, false, func(tx connection.DbTransaction) error {
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
							"values": state.Values,
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
	super.Rdbw.XDel("icinga:history:stream:" + objectType + ":" + historyType, storedEntryIds...)

	log.Debugf("%d %s %s history entries synced", len(storedEntryIds) - brokenEntries, objectType, historyType)
	log.Debugf("%d %s %s history entries broken", brokenEntries, objectType, historyType)
}

//Removes one redis.XMessage at given index from given slice and returns the resulting slice
func removeEntryFromEntriesSlice(s []redis.XMessage, i int) []redis.XMessage {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

