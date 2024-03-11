CREATE OR REPLACE FUNCTION get_sla_ok_percent(
  in_host_id bytea20,
  in_service_id bytea20,
  in_start_time biguint,
  in_end_time biguint
)
RETURNS decimal(7, 4)
LANGUAGE plpgsql
STABLE
PARALLEL RESTRICTED
AS $$
DECLARE
  last_event_time biguint := in_start_time;
  last_hard_state tinyuint;
  active_downtimes uint := 0;
  problem_time biguint := 0;
  total_time biguint;
  row record;
BEGIN
  IF in_end_time <= in_start_time THEN
    RAISE 'end time must be greater than start time';
  END IF;

  total_time := in_end_time - in_start_time;

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
    last_hard_state := 0;
  END IF;

  FOR row IN
    (
      -- all downtime_start events before the end of the SLA interval
      -- for downtimes that overlap the SLA interval in any way
      SELECT
        GREATEST(downtime_start, in_start_time) AS event_time,
        'downtime_start' AS event_type,
        1 AS event_prio,
        NULL::tinyuint AS hard_state,
        NULL::tinyuint AS previous_hard_state
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
        NULL::tinyuint AS hard_state,
        NULL::tinyuint AS previous_hard_state
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
        NULL::tinyuint AS hard_state,
        NULL::tinyuint AS previous_hard_state
    )
    ORDER BY event_time, event_prio
  LOOP
    IF row.previous_hard_state = 99 THEN
      total_time := total_time - (row.event_time - last_event_time);
    ELSEIF ((in_service_id IS NULL AND last_hard_state > 0) OR (in_service_id IS NOT NULL AND last_hard_state > 1))
      AND last_hard_state != 99
      AND active_downtimes = 0
    THEN
      problem_time := problem_time + row.event_time - last_event_time;
    END IF;

    last_event_time := row.event_time;
    IF row.event_type = 'state_change' THEN
      last_hard_state := row.hard_state;
    ELSEIF row.event_type = 'downtime_start' THEN
      active_downtimes := active_downtimes + 1;
    ELSEIF row.event_type = 'downtime_end' THEN
      active_downtimes := active_downtimes - 1;
    END IF;
  END LOOP;

  RETURN (100 * (total_time - problem_time)::decimal / total_time)::decimal(7, 4);
END;
$$;

CREATE INDEX CONCURRENTLY idx_history_event_time_event_type ON history(event_time, event_type);
COMMENT ON INDEX idx_history_event_time_event_type IS 'History filtered/ordered by event_time/event_type';

DROP INDEX idx_history_event_time;
