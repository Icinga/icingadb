SELECT sh.statehistory_id, UNIX_TIMESTAMP(sh.state_time) state_time, sh.state_time_usec, sh.state,
  sh.state_type, sh.current_check_attempt, sh.max_check_attempts, sh.last_state, sh.last_hard_state,
  sh.output, sh.long_output, sh.check_source, o.objecttype_id, o.name1, COALESCE(o.name2, '') name2
FROM icinga_statehistory sh USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=sh.object_id
WHERE sh.statehistory_id <= :cache_limit AND sh.statehistory_id > :checkpoint -- where we were interrupted
ORDER BY sh.statehistory_id -- allows computeProgress() not to check all IDO rows for whether migrated
LIMIT :bulk
