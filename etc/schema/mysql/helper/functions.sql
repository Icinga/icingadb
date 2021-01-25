-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

DROP FUNCTION IF EXISTS unix_timestamp_ms;

CREATE FUNCTION unix_timestamp_ms()
  RETURNS BIGINT UNSIGNED
  NO SQL
  RETURN FLOOR((UNIX_TIMESTAMP(NOW(3)) * 1000));


DROP FUNCTION IF EXISTS reports_get_sla_ok_percent;

DELIMITER //

CREATE FUNCTION reports_get_sla_ok_percent(
  object_type enum('host', 'service'),
  object_id binary(20),
  start_time bigint unsigned,
  end_time bigint unsigned
)
RETURNS decimal(7, 4)
READS SQL DATA
BEGIN
  DECLARE result decimal(7, 4);
  DECLARE cur_event_time bigint unsigned;
  DECLARE cur_event_type enum('state_change', 'downtime_start', 'downtime_end', 'end');
  DECLARE cur_hard_state tinyint unsigned;
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
      SELECT GREATEST(downtime_start, start_time) AS event_time, 'downtime_start' AS event_type, NULL AS hard_state
      FROM sla_history_downtime d
      WHERE d.object_type = object_type
        AND d.object_id = object_id
        AND d.downtime_start < end_time
        AND d.downtime_end >= start_time
    ) UNION ALL (
      -- all downtime_end events before the end of the SLA interval
      -- for downtimes that overlap the SLA interval in any way
      SELECT downtime_end AS event_time, 'downtime_end' AS event_type, NULL AS hard_state
      FROM sla_history_downtime d
      WHERE d.object_type = object_type
        AND d.object_id = object_id
        AND d.downtime_start < end_time
        AND d.downtime_end >= start_time
        AND d.downtime_end < end_time
    ) UNION ALL (
      -- state event at the beginning of the SLA interval or the newest one from before
      SELECT start_time AS event_time, 'state_change' AS event_type, hard_state
      FROM sla_history_state s
      WHERE s.object_type = object_type
        AND s.object_id = object_id
        AND s.event_time <= start_time
      ORDER BY s.event_time DESC
      LIMIT 1
    ) UNION ALL (
      -- all state events strictly in interval
      SELECT event_time, 'state_change' AS event_type, hard_state
      FROM sla_history_state s
      WHERE s.object_type = object_type
        AND s.object_id = object_id
        AND s.event_time > start_time
        AND s.event_time < end_time
    ) UNION ALL (
      -- end event to keep loop simple, values are not used
      SELECT end_time AS event_time, 'end' AS event_type, NULL AS hard_state
    ) ORDER BY event_time, CASE
      WHEN event_type = 'state_change' THEN 0
      WHEN event_type = 'downtime_start' THEN 1
      WHEN event_type = 'downtime_end' THEN 2
      ELSE 3
    END
  ;
  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

  SET problem_time = 0;
  SET total_time = end_time - start_time;
  SET last_event_time = start_time;
  SET active_downtimes = 0;

  SET done = 0;
  OPEN cur;
  read_loop: LOOP
    FETCH cur INTO cur_event_time, cur_event_type, cur_hard_state;
    IF done THEN
      LEAVE read_loop;
    END IF;

    IF ((object_type = 'host' AND last_hard_state > 0) OR (object_type = 'service' AND last_hard_state > 1))
      AND active_downtimes = 0
    THEN
      SET problem_time = problem_time + cur_event_time - last_event_time;
    END IF;

    SET last_event_time = cur_event_time;
    IF cur_event_type = 'state_change' THEN
      SET last_hard_state = cur_hard_state;
    ELSEIF cur_event_type = 'downtime_start' THEN
      SET active_downtimes = active_downtimes + 1;
    ELSEIF cur_event_type = 'downtime_end' THEN
      SET active_downtimes = active_downtimes - 1;
    END IF;
  END LOOP;
  CLOSE cur;

  SET result = 100 * (total_time - problem_time) / total_time;
  RETURN result;
END//

DELIMITER ;
