// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package history

import (
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis/v7"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"strconv"
	"sync/atomic"
	"time"
)
const(
	limit = 65535
	biglimit = 16777215
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

			text, truncated := utils.TruncText(values["text"].(string), limit)
			if truncated {
				log.WithFields(log.Fields{
					"Table": "notification_history",
					"Column": "text",
					"id": utils.DecodeChecksum(id[:]),
				}).Infof("Truncated notification message to 64KB")
			}

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
				text,
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

			outputVal, truncated := utils.TruncText(utils.DefaultIfNil(values["output"], "").(string), biglimit)
			if truncated {
				log.WithFields(log.Fields{
					"Table": "state_history",
					"Column": "output",
					"id": utils.DecodeChecksum(id[:]),
				}).Infof("Truncated plugin output to 64KB")
			}

			longOutputVal, truncated := utils.TruncText(utils.DefaultIfNil(values["long_output"], "").(string), biglimit)
			if truncated {
				log.WithFields(log.Fields{
					"Table": "state_history",
					"Column": "long_output",
					"id": utils.DecodeChecksum(id[:]),
				}).Infof("Truncated long plugin output to 64KB")
			}

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
				outputVal,
				longOutputVal,
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
			`author, cancelled_by, comment, is_flexible, flexible_duration, scheduled_start_time, scheduled_end_time, start_time, end_time, has_been_cancelled, trigger_time, cancel_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			var triggeredById interface{}
			if values["triggered_by_id"] != nil {
				triggeredById = utils.EncodeChecksum(values["triggered_by_id"].(string))
			}

			comment, truncated := utils.TruncText(values["comment"].(string), limit)
			if truncated {
				log.WithFields(log.Fields{
					"Table": "downtime_history",
					"Column": "comment",
					"downtime_id": values["downtime_id"].(string),
				}).Infof("Truncated downtime comment message to 64KB")
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
				values["cancelled_by"],
				comment,
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
				if values["has_been_cancelled"].(string) == "1" {
					eventTime = values["cancel_time"].(string)
				} else {
					eventTime = values["end_time"].(string)
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
			`removed_by, comment, entry_type, is_persistent, is_sticky, expire_time, remove_time, has_been_removed)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			comment, truncated := utils.TruncText(values["comment"].(string), limit)
			if truncated {
				log.WithFields(log.Fields{
					"Table": "comment_history",
					"Column": "comment",
					"comment_id": values["comment_id"].(string),
				}).Infof("Truncated comment message to 64KB")
			}

			data := []interface{}{
				utils.EncodeChecksum(values["comment_id"].(string)),
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["entry_time"],
				values["author"],
				values["removed_by"],
				comment,
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
		`INSERT INTO flapping_history (id, environment_id, endpoint_id, object_type, host_id, service_id, start_time, end_time, ` +
			`percent_state_change_start, percent_state_change_end, flapping_threshold_low, flapping_threshold_high)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?)` +
			`ON DUPLICATE KEY UPDATE ` +
			`start_time=IFNULL(NULLIF(start_time, 0), VALUES(start_time)),` +
			`end_time=IFNULL(NULLIF(end_time, 0), VALUES(end_time)),` +
			`percent_state_change_start=IFNULL(NULLIF(percent_state_change_start, 0), VALUES(percent_state_change_start)),` +
			`percent_state_change_end=IFNULL(NULLIF(percent_state_change_end, 0), VALUES(percent_state_change_end)),` +
			`flapping_threshold_low=IFNULL(NULLIF(flapping_threshold_low, 0), VALUES(flapping_threshold_low)),` +
			`flapping_threshold_high=IFNULL(NULLIF(flapping_threshold_high, 0), VALUES(flapping_threshold_high))`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id, notification_history_id,` +
			`state_history_id, downtime_history_id, comment_history_id, flapping_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			data := []interface{}{
				utils.EncodeChecksum(values["id"].(string)),
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["start_time"],
				values["end_time"],
				values["percent_state_change_start"],
				values["percent_state_change_end"],
				values["flapping_threshold_low"],
				values["flapping_threshold_high"],
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))

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
				utils.EncodeChecksum(values["id"].(string)),
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
		`INSERT INTO acknowledgement_history (id, environment_id, endpoint_id, object_type, host_id, service_id, set_time, clear_time,` +
			`author, cleared_by, comment, expire_time, is_sticky, is_persistent)` +
			`VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)` +
			`ON DUPLICATE KEY UPDATE ` +
			`set_time=IFNULL(NULLIF(set_time, 0), VALUES(set_time)),` +
			`clear_time=IFNULL(NULLIF(clear_time, 0), VALUES(clear_time)),` +
			`author=IFNULL(NULLIF(author, ''), VALUES(author)),` +
			`cleared_by=IFNULL(NULLIF(cleared_by, ''), VALUES(cleared_by)),` +
			`comment=IFNULL(NULLIF(comment, ''), VALUES(comment)),` +
			`expire_time=IFNULL(NULLIF(expire_time, 0), VALUES(expire_time)),` +
			`is_sticky=IFNULL(NULLIF(is_sticky, 'n'), VALUES(is_sticky)),` +
			`is_persistent=IFNULL(NULLIF(is_persistent, 'n'), VALUES(is_persistent))`,
		`REPLACE INTO history (id, environment_id, endpoint_id, object_type, host_id, service_id,` +
			` acknowledgement_history_id, event_type, event_time)` +
			`VALUES (?,?,?,?,?,?,?,?,?)`,
	}

	dataFunctions := []func(values map[string]interface{}) []interface{}{
		func(values map[string]interface{}) []interface{} {
			comment, truncated := utils.TruncText(utils.DefaultIfNil(values["comment"], "").(string), limit)
			if truncated {
				log.WithFields(log.Fields{
					"Table": "acknowledgement_history",
					"Column": "comment",
					"id": values["id"].(string),
				}).Infof("Truncated acknowledgement comment message to 64KB")
			}

			data := []interface{}{
				utils.EncodeChecksum(values["id"].(string)),
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				values["set_time"],
				values["clear_time"],
				utils.DefaultIfNil(values["author"], ""),
				values["cleared_by"],
				utils.DefaultIfNil(interface{}(comment), ""),
				values["expire_time"],
				utils.RedisIntToDBBoolean(values["is_sticky"]),
				utils.RedisIntToDBBoolean(values["is_persistent"]),
			}

			return data
		},
		func(values map[string]interface{}) []interface{} {
			eventId := uuid.MustParse(values["event_id"].(string))

			var eventTime string
			switch values["event_type"] {
			case "ack_set":
				eventTime = values["set_time"].(string)
			case "ack_clear":
				eventTime = values["clear_time"].(string)
			}

			data := []interface{}{
				eventId[:],
				super.EnvId,
				utils.DecodeHexIfNotNil(values["endpoint_id"]),
				values["object_type"].(string),
				utils.EncodeChecksum(values["host_id"].(string)),
				utils.DecodeHexIfNotNil(values["service_id"]),
				utils.EncodeChecksum(values["id"].(string)),
				values["event_type"],
				eventTime,
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
