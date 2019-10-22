package nullrows

import (
	"fmt"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
)

func InsertNullRows(super *supervisor.Supervisor) {
	for {
		if super.EnvId != nil {
			break
		}
		time.Sleep(time.Second)
	}

	var emptyUUID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
	var emptyID = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	execFunc := func(name string, query string, args ...interface{}) {
		_, err := super.Dbw.Db.Exec(query, args...)

		if err != nil {
			log.WithError(err).Errorf("Could not insert NULL row for %s table", name)
		}
	}

	// Host
	execFunc(
		"host",
		"REPLACE INTO host(id, environment_id, name_checksum, properties_checksum, customvars_checksum, groups_checksum, name, name_ci, display_name, address, address6, address_bin, address6_bin, checkcommand, checkcommand_id, max_check_attempts, check_timeperiod, check_timeperiod_id, check_timeout, check_interval, check_retry_interval, active_checks_enabled, passive_checks_enabled, event_handler_enabled, notifications_enabled, flapping_enabled, flapping_threshold_low, flapping_threshold_high, perfdata_enabled, eventcommand, eventcommand_id, is_volatile, action_url_id, notes_url_id, notes, icon_image_id, icon_image_alt, zone, zone_id, command_endpoint, command_endpoint_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		emptyID, super.EnvId, emptyID, emptyID, emptyID, emptyID, "", "", "", "", "", []byte{0, 0, 0, 0}, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "", emptyID, 0, "", emptyID, 0, 0, 0, "y", "y", "y", "y", "y", 0, 0, "y", "", emptyID, "y", emptyID, emptyID, "", emptyID, "", "", emptyID, "", emptyID,
	)

	// Service
	execFunc(
		"service",
		"REPLACE INTO service(id, environment_id, name_checksum, properties_checksum, customvars_checksum, groups_checksum, host_id, name, name_ci, display_name, checkcommand, checkcommand_id, max_check_attempts, check_timeperiod, check_timeperiod_id, check_timeout, check_interval, check_retry_interval, active_checks_enabled, passive_checks_enabled, event_handler_enabled, notifications_enabled, flapping_enabled, flapping_threshold_low, flapping_threshold_high, perfdata_enabled, eventcommand, eventcommand_id, is_volatile, action_url_id, notes_url_id, notes, icon_image_id, icon_image_alt, zone, zone_id, command_endpoint, command_endpoint_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		emptyID, super.EnvId, emptyID, emptyID, emptyID, emptyID, emptyID, "", "", "", "", emptyID, 0, "", emptyID, 0, 0, 0, "y", "y", "y", "y", "y", 0, 0, "y", "", emptyID, "y", emptyID, emptyID, "", emptyID, "", "", emptyID, "", emptyID,
	)

	for _, objectType := range []string{"host", "service"} {
		// *_comment_history
		execFunc(
			objectType + "_comment_history",
			fmt.Sprintf("REPLACE INTO %s_comment_history(comment_id, environment_id, %s_id, entry_time, author, comment, entry_type, is_persistent, expire_time, deletion_time) VALUES (?,?,?,?,?,?,?,?,?,?)", objectType, objectType),
			emptyID, super.EnvId, emptyID, 0, "", "", "comment", "y", 0, 0,
		)

		// *_downtime_history
		execFunc(
			objectType + "_downtime_history",
			fmt.Sprintf("REPLACE INTO %s_downtime_history(downtime_id, environment_id, %s_id, triggered_by_id, entry_time, author, comment, is_fixed, duration, scheduled_start_time, scheduled_end_time, was_started, actual_start_time, actual_end_time, was_cancelled, is_in_effect, trigger_time, deletion_time) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", objectType, objectType),
			emptyID, super.EnvId, emptyID, emptyID, 0, "", "", "y", 0, 0, 0, "y", 0, 0, "y", "y", 0, 0,
		)

		// *_flapping_history
		execFunc(
			objectType + "_flapping_history",
			fmt.Sprintf("REPLACE INTO %s_flapping_history(id, environment_id, %s_id, change_time, change_type, percent_state_change, flapping_threshold_low, flapping_threshold_high) VALUES (?,?,?,?,?,?,?,?)", objectType, objectType),
			emptyUUID[:], super.EnvId, emptyID, 0, "start", 0, 0, 0,
		)

		// *_notification_history
		execFunc(
			objectType + "_notification_history",
			fmt.Sprintf("REPLACE INTO %s_notification_history(id, environment_id, %s_id, notification_id, type, send_time, state, output, long_output, users_notified) VALUES (?,?,?,?,?,?,?,?,?,?)", objectType, objectType),
			emptyUUID[:], super.EnvId, emptyID, emptyID, 0, 0, 0, "", "", 0,
		)

		// *_state_history
		execFunc(
			objectType + "_state_history",
			fmt.Sprintf("REPLACE INTO %s_state_history(id, environment_id, %s_id, change_time, state_type, soft_state, hard_state, attempt, last_soft_state, last_hard_state, output, long_output, max_check_attempts) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", objectType, objectType),
			emptyUUID[:], super.EnvId, emptyID, 0, "hard", 0, 0, 0, 0, 0, "", "", 0,
		)
	}

	log.Info("Inserted \"NULL\" rows")
}