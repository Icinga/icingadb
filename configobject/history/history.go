// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package history

import (
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"strconv"
	"sync/atomic"
	"time"
)

var mysqlObservers = struct {
	state            prometheus.Observer
	notification     prometheus.Observer
	usernotification prometheus.Observer
	downtime         prometheus.Observer
	comment          prometheus.Observer
	flapping         prometheus.Observer
	acknowledgement  prometheus.Observer
}{
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into state_history"),
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into notification_history"),
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into usernotification_history"),
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into downtime_history"),
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into comment_history"),
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into flapping_history"),
	connection.DbIoSeconds.WithLabelValues("mysql", "replace into acknowledgement_history"),
}

var historyCounter = struct {
	state            uint64
	notification     uint64
	usernotification uint64
	downtime         uint64
	comment          uint64
	flapping         uint64
	acknowledgement  uint64
}{}

func printAndResetHistoryCounter(counter *uint64, historyType string) {
	amount := atomic.SwapUint64(counter, 0)

	if amount > 0 {
		log.Infof("Added %d %s history entries in the last 20 seconds", amount, historyType)
	}
}

// logHistoryCounters logs the amount of history entries added every 20 seconds.
func logHistoryCounters() {
	every20s := time.NewTicker(time.Second * 20)
	defer every20s.Stop()

	for {
		<-every20s.C
		printAndResetHistoryCounter(&historyCounter.state, "state")
		printAndResetHistoryCounter(&historyCounter.notification, "notification")
		printAndResetHistoryCounter(&historyCounter.usernotification, "usernotification")
		printAndResetHistoryCounter(&historyCounter.downtime, "downtime")
		printAndResetHistoryCounter(&historyCounter.comment, "comment")
		printAndResetHistoryCounter(&historyCounter.flapping, "flapping")
		printAndResetHistoryCounter(&historyCounter.acknowledgement, "acknowledgement")
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
		acknowledgementHistoryWorker,
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
			"send_time, state, previous_hard_state, author, `text`, users_notified)" +
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				utils.EncodeChecksum(values["notification_id"].(string)),
				utils.NotificationTypesToDbEnumString[values["type"].(string)],
				values["send_time"],
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				notificationHistoryId[:],
				nil,
				nil,
				nil,
				nil,
				values["event_type"],
				values["send_time"],
			}

			return data
		},
	}

	historyWorker(super, "notification", statements, dataFunctions, mysqlObservers.notification, &historyCounter.notification)
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

	historyWorker(super, "usernotification", statements, dataFunctions, mysqlObservers.usernotification, &historyCounter.usernotification)
}

func stateHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO state_history (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time, state_type,` +
			`soft_state, hard_state, previous_soft_state, previous_hard_state, attempt, output, long_output, max_check_attempts, check_source)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["event_time"],
				utils.IcingaStateTypeToString(float32(stateType)),
				values["soft_state"],
				values["hard_state"],
				values["previous_soft_state"],
				values["previous_hard_state"],
				values["attempt"],
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				nil,
				stateHistoryId[:],
				nil,
				nil,
				nil,
				values["event_type"],
				values["event_time"],
			}

			return data
		},
	}

	historyWorker(super, "state", statements, dataFunctions, mysqlObservers.state, &historyCounter.state)
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
				utils.EncodeChecksum(values["host_id"].(string)),
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				nil,
				nil,
				downtimeHistoryId,
				nil,
				nil,
				values["event_type"],
				eventTime,
			}

			return data
		},
	}

	historyWorker(super, "downtime", statements, dataFunctions, mysqlObservers.downtime, &historyCounter.downtime)
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
				utils.EncodeChecksum(values["host_id"].(string)),
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				nil,
				nil,
				nil,
				commentHistoryId,
				nil,
				values["event_type"],
				eventTime,
			}

			return data
		},
	}

	historyWorker(super, "comment", statements, dataFunctions, mysqlObservers.comment, &historyCounter.comment)
}

func flappingHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO flapping_history (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time,` +
			`percent_state_change, flapping_threshold_low, flapping_threshold_high)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?)`,
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["event_time"],
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
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				nil,
				nil,
				nil,
				nil,
				flappingHistoryId[:],
				values["event_type"],
				eventTime,
			}

			return data
		},
	}

	historyWorker(super, "flapping", statements, dataFunctions, mysqlObservers.flapping, &historyCounter.flapping)
}

func acknowledgementHistoryWorker(super *supervisor.Supervisor) {
	statements := []string{
		`REPLACE INTO acknowledgement_history (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time,` +
			`author, comment, expire_time, is_sticky, is_persistent)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id,` +
			` acknowledgement_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))
			data := []interface{}{
				id[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["event_time"],
				values["author"],
				values["comment"],
				values["expire_time"],
				utils.RedisIntToDBBoolean(values["is_sticky"]),
				utils.RedisIntToDBBoolean(values["is_persistent"]),
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			id := uuid.MustParse(values["id"].(string))

			data := []interface{}{
				id[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				id[:],
				values["event_type"],
				values["event_time"],
			}

			return data
		},
	}

	historyWorker(super, "acknowledgement", statements, dataFunctions, mysqlObservers.acknowledgement, &historyCounter.acknowledgement)
}

func historyWorker(super *supervisor.Supervisor, historyType string, preparedStatements []string, dataFunctions []func(map[string]interface{}) []interface{}, observer prometheus.Observer, counter *uint64) {
	if super.EnvId == nil {
		log.Debug(historyType + "History: Waiting for EnvId to be set")
		time.Sleep(time.Second)
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
	atomic.AddUint64(counter, 1)

	log.Debugf("%d %s history entries synced", count, historyType)
	log.Debugf("%d %s history entries broken", brokenEntries, historyType)
}

// removeEntryFromEntriesSlice removes one redis.XMessage at given index from given slice and returns the resulting slice.
func removeEntryFromEntriesSlice(s []redis.XMessage, i int) []redis.XMessage {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}
