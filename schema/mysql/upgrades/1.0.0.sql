DROP FUNCTION IF EXISTS get_sla_ok_percent;
DELIMITER //
CREATE FUNCTION get_sla_ok_percent(
  in_host_id binary(20),
  in_service_id binary(20),
  in_start_time bigint unsigned,
  in_end_time bigint unsigned
)
RETURNS decimal(7, 4)
READS SQL DATA
BEGIN
  DECLARE result decimal(7, 4);
  DECLARE row_event_time bigint unsigned;
  DECLARE row_event_type enum('state_change', 'downtime_start', 'downtime_end', 'end');
  DECLARE row_event_prio int;
  DECLARE row_hard_state tinyint unsigned;
  DECLARE row_previous_hard_state tinyint unsigned;
  DECLARE last_event_time bigint unsigned;
  DECLARE last_hard_state tinyint unsigned;
  DECLARE active_downtimes int unsigned;
  DECLARE problem_time bigint unsigned;
  DECLARE total_time bigint unsigned;
  DECLARE done int;
  DECLARE cur CURSOR FOR
    (
      -- all downtime_start events before the end of the SLA interval
      -- for downtimes that overlap the SLA interval in any way
      SELECT
        GREATEST(downtime_start, in_start_time) AS event_time,
        'downtime_start' AS event_type,
        1 AS event_prio,
        NULL AS hard_state,
        NULL AS previous_hard_state
      FROM sla_history_downtime d
      WHERE d.host_id = in_host_id
        AND ((in_service_id IS NULL AND d.service_id IS NULL) OR d.service_id = in_service_id)
        AND d.downtime_start < in_end_time
        AND d.downtime_end >= in_start_time
    ) UNION ALL (
      -- all downtime_end events before the end of the SLA interval
      -- for downtimes that overlap the SLA interval in any way
      SELECT
        downtime_end AS event_time,
        'downtime_end' AS event_type,
        2 AS event_prio,
        NULL AS hard_state,
        NULL AS previous_hard_state
      FROM sla_history_downtime d
      WHERE d.host_id = in_host_id
        AND ((in_service_id IS NULL AND d.service_id IS NULL) OR d.service_id = in_service_id)
        AND d.downtime_start < in_end_time
        AND d.downtime_end >= in_start_time
        AND d.downtime_end < in_end_time
    ) UNION ALL (
      -- all state events strictly in interval
      SELECT
        event_time,
        'state_change' AS event_type,
        0 AS event_prio,
        hard_state,
        previous_hard_state
      FROM sla_history_state s
      WHERE s.host_id = in_host_id
        AND ((in_service_id IS NULL AND s.service_id IS NULL) OR s.service_id = in_service_id)
        AND s.event_time > in_start_time
        AND s.event_time < in_end_time
    ) UNION ALL (
      -- end event to keep loop simple, values are not used
      SELECT
        in_end_time AS event_time,
        'end' AS event_type,
        3 AS event_prio,
        NULL AS hard_state,
        NULL AS previous_hard_state
    )
    ORDER BY event_time, event_prio;
  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

  IF in_end_time <= in_start_time THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'end time must be greater than start time';
  END IF;

  -- Use the latest event at or before the beginning of the SLA interval as the initial state.
  SELECT hard_state INTO last_hard_state
  FROM sla_history_state s
  WHERE s.host_id = in_host_id
    AND ((in_service_id IS NULL AND s.service_id IS NULL) OR s.service_id = in_service_id)
    AND s.event_time <= in_start_time
  ORDER BY s.event_time DESC
  LIMIT 1;

  -- If this does not exist, use the previous state from the first event after the beginning of the SLA interval.
  IF last_hard_state IS NULL THEN
    SELECT previous_hard_state INTO last_hard_state
    FROM sla_history_state s
    WHERE s.host_id = in_host_id
      AND ((in_service_id IS NULL AND s.service_id IS NULL) OR s.service_id = in_service_id)
      AND s.event_time > in_start_time
    ORDER BY s.event_time ASC
    LIMIT 1;
  END IF;

  -- If this also does not exist, use the current host/service state.
  IF last_hard_state IS NULL THEN
    IF in_service_id IS NULL THEN
      SELECT hard_state INTO last_hard_state
      FROM host_state s
      WHERE s.host_id = in_host_id;
    ELSE
      SELECT hard_state INTO last_hard_state
      FROM service_state s
      WHERE s.host_id = in_host_id
        AND s.service_id = in_service_id;
    END IF;
  END IF;

  IF last_hard_state IS NULL THEN
    SET last_hard_state = 0;
  END IF;

  SET problem_time = 0;
  SET total_time = in_end_time - in_start_time;
  SET last_event_time = in_start_time;
  SET active_downtimes = 0;

  SET done = 0;
  OPEN cur;
  read_loop: LOOP
    FETCH cur INTO row_event_time, row_event_type, row_event_prio, row_hard_state, row_previous_hard_state;
    IF done THEN
      LEAVE read_loop;
    END IF;

    IF row_previous_hard_state = 99 THEN
      SET total_time = total_time - (row_event_time - last_event_time);
    ELSEIF ((in_service_id IS NULL AND last_hard_state > 0) OR (in_service_id IS NOT NULL AND last_hard_state > 1))
      AND last_hard_state != 99
      AND active_downtimes = 0
    THEN
      SET problem_time = problem_time + row_event_time - last_event_time;
    END IF;

    SET last_event_time = row_event_time;
    IF row_event_type = 'state_change' THEN
      SET last_hard_state = row_hard_state;
    ELSEIF row_event_type = 'downtime_start' THEN
      SET active_downtimes = active_downtimes + 1;
    ELSEIF row_event_type = 'downtime_end' THEN
      SET active_downtimes = active_downtimes - 1;
    END IF;
  END LOOP;
  CLOSE cur;

  SET result = 100 * (total_time - problem_time) / total_time;
  RETURN result;
END//
DELIMITER ;

ALTER TABLE hostgroup
    DROP INDEX idx_hostroup_name,
    ADD INDEX idx_hostgroup_name (name) COMMENT 'Host/service/host group list filtered by host group name';

ALTER TABLE notification_history
  MODIFY `text` longtext NOT NULL;

ALTER TABLE host_state
  ADD COLUMN previous_soft_state tinyint unsigned NOT NULL AFTER hard_state;

ALTER TABLE service_state
  ADD COLUMN previous_soft_state tinyint unsigned NOT NULL AFTER hard_state;

ALTER TABLE checkcommand_argument
  ADD COLUMN `separator` varchar(255) DEFAULT NULL AFTER set_if;

ALTER TABLE eventcommand_argument
  ADD COLUMN `separator` varchar(255) DEFAULT NULL AFTER set_if;

ALTER TABLE notificationcommand_argument
  ADD COLUMN `separator` varchar(255) DEFAULT NULL AFTER set_if;

ALTER TABLE acknowledgement_history
  ADD index idx_acknowledgement_history_clear_time (clear_time) COMMENT 'Filter for history retention';

ALTER TABLE comment_history
  ADD index idx_comment_history_remove_time (remove_time) COMMENT 'Filter for history retention';

ALTER TABLE downtime_history
  ADD index idx_downtime_history_end_time (end_time) COMMENT 'Filter for history retention';

ALTER TABLE flapping_history
  ADD index idx_flapping_history_end_time (end_time) COMMENT 'Filter for history retention';

ALTER TABLE state_history
  ADD index idx_state_history_event_time (event_time) COMMENT 'Filter for history retention';

ALTER TABLE icon_image
  DROP PRIMARY KEY,
  MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + icon_image)',
  ADD PRIMARY KEY (id);

ALTER TABLE action_url
  DROP PRIMARY KEY,
  MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + action_url)',
  ADD PRIMARY KEY (id);

ALTER TABLE notes_url
  DROP PRIMARY KEY,
  MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notes_url)',
  ADD PRIMARY KEY (id);

ALTER TABLE customvar
  MODIFY name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL;

ALTER TABLE customvar_flat
  MODIFY flatname varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Path converted with `.` and `[ ]`';

ALTER TABLE host
  CHANGE checkcommand checkcommand_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'checkcommand.name';

ALTER TABLE host
  CHANGE check_timeperiod
    check_timeperiod_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'timeperiod.name';

ALTER TABLE host
  CHANGE eventcommand eventcommand_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'eventcommand.name';

ALTER TABLE host
  CHANGE zone zone_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'zone.name';

ALTER TABLE host
  CHANGE command_endpoint command_endpoint_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'endpoint.name';

ALTER TABLE service
  CHANGE checkcommand checkcommand_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'checkcommand.name';

ALTER TABLE service
  CHANGE check_timeperiod
    check_timeperiod_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'timeperiod.name';

ALTER TABLE service
  CHANGE eventcommand eventcommand_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'eventcommand.name';

ALTER TABLE service
  CHANGE zone zone_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'zone.name';

ALTER TABLE service
  CHANGE command_endpoint command_endpoint_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'endpoint.name';

CREATE TABLE sla_history_state (
  id binary(20) NOT NULL COMMENT 'state_history.id (may reference already deleted rows)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  event_time bigint unsigned NOT NULL COMMENT 'unix timestamp the event occurred',
  hard_state TINYINT UNSIGNED NOT NULL COMMENT 'hard state after this event',
  previous_hard_state TINYINT UNSIGNED NOT NULL COMMENT 'hard state before this event',

  PRIMARY KEY (id),

  INDEX idx_sla_history_state_event (host_id, service_id, event_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

INSERT INTO sla_history_state
  (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time, hard_state, previous_hard_state)
  SELECT id, environment_id, endpoint_id, object_type, host_id, service_id, event_time, hard_state, previous_hard_state
  FROM state_history
  WHERE state_type = 'hard'
  ON DUPLICATE KEY UPDATE sla_history_state.id = sla_history_state.id;

CREATE TABLE sla_history_downtime (
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  downtime_id binary(20) NOT NULL COMMENT 'downtime.id (may reference already deleted rows)',
  downtime_start BIGINT UNSIGNED NOT NULL COMMENT 'start time of the downtime',
  downtime_end BIGINT UNSIGNED NOT NULL COMMENT 'end time of the downtime',

  PRIMARY KEY (downtime_id),

  INDEX idx_sla_history_downtime_event (host_id, service_id, downtime_start, downtime_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

INSERT INTO sla_history_downtime
  (environment_id, endpoint_id, object_type, host_id, service_id, downtime_id, downtime_start, downtime_end)
  SELECT environment_id, endpoint_id, object_type, host_id, service_id, downtime_id,
    start_time AS downtime_start, IF(has_been_cancelled = 'y', cancel_time, end_time) AS downtime_end
  FROM downtime_history
  ON DUPLICATE KEY UPDATE sla_history_downtime.downtime_id = sla_history_downtime.downtime_id;

INSERT INTO icingadb_schema (version, TIMESTAMP)
  VALUES (3, CURRENT_TIMESTAMP() * 1000);
