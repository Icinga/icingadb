SELECT
  fh.flappinghistory_id,
  UNIX_TIMESTAMP(fh.event_time) AS event_time,
  fh.event_time_usec,
  fh.event_type,
  fh.percent_state_change,
  fh.low_threshold,
  fh.high_threshold,
  o.objecttype_id,
  o.name1,
  COALESCE(o.name2, '') AS name2
FROM icinga_flappinghistory fh USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=fh.object_id
WHERE fh.flappinghistory_id BETWEEN :fromid AND :toid
AND fh.flappinghistory_id > :checkpoint -- where we were interrupted
ORDER BY fh.flappinghistory_id -- this way we know what has already been migrated from just the last row's ID
LIMIT :bulk
