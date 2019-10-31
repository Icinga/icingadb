package nullrows

import (
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

	// comment_history
	execFunc(
		"comment_history",
		"REPLACE INTO comment_history(comment_id, environment_id, object_type, host_id, service_id, entry_time, author, comment, entry_type, is_persistent, is_sticky, expire_time, remove_time) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		emptyID, super.EnvId, "host", nil, nil, 0, "", "", "comment", "y", "y", 0, 0,
	)

	// downtime_history
	execFunc(
		"downtime_history",
		"REPLACE INTO downtime_history(downtime_id, environment_id, triggered_by_id, object_type, host_id, service_id, entry_time, author, comment, is_flexible, flexible_duration, scheduled_start_time, scheduled_end_time, start_time, end_time, has_been_cancelled, trigger_time, cancel_time) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		emptyID, super.EnvId, emptyID, "host", nil, nil, 0, "", "", "y", 0, 0, 0, 0, 0, "y", 0, 0,
	)

	// flapping_history
	execFunc(
		"flapping_history",
		"REPLACE INTO flapping_history(id, environment_id, object_type, host_id, service_id, event_time, event_type, percent_state_change, flapping_threshold_low, flapping_threshold_high) VALUES (?,?,?,?,?,?,?,?,?,?)",
		emptyUUID[:], super.EnvId, "host", nil, nil, 0, "flapping_start", 0, 0, 0,
	)

	// notification_history
	execFunc(
		"notification_history",
		"REPLACE INTO notification_history(id, environment_id, object_type, host_id, service_id, notification_id, type, event_time, state, previous_hard_state, author, `text`, users_notified) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		emptyUUID[:], super.EnvId, "host", nil, nil, emptyID, 0, 0, 0, 0, "", "", 0,
	)

	// state_history
	execFunc(
		"state_history",
		"REPLACE INTO state_history(id, environment_id, object_type, host_id, service_id, event_time, state_type, soft_state, hard_state, previous_hard_state, attempt, last_soft_state, last_hard_state, output, long_output, max_check_attempts) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		emptyUUID[:], super.EnvId, "host", nil, nil, 0, "hard", 0, 0, 0, 0, 0, 0, "", "", 0,
	)

	log.Info("Inserted \"NULL\" rows")
}
