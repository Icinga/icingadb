SELECT
  dh.downtimehistory_id,
  UNIX_TIMESTAMP(dh.entry_time) AS entry_time,
  dh.author_name,
  dh.comment_data,
  dh.is_fixed,
  dh.duration,
  UNIX_TIMESTAMP(dh.scheduled_start_time) AS scheduled_start_time,
  COALESCE(UNIX_TIMESTAMP(dh.scheduled_end_time), 0) AS scheduled_end_time,
  dh.was_started,
  COALESCE(UNIX_TIMESTAMP(dh.actual_start_time), 0) AS actual_start_time,
  dh.actual_start_time_usec,
  COALESCE(UNIX_TIMESTAMP(dh.actual_end_time), 0) AS actual_end_time,
  dh.actual_end_time_usec,
  dh.was_cancelled,
  COALESCE(UNIX_TIMESTAMP(dh.trigger_time), 0) AS trigger_time,
  COALESCE(dh.name, CONCAT(o.name1, '!', COALESCE(o.name2, ''), '!', dh.downtimehistory_id, '-', dh.object_id)) AS name,
  o.objecttype_id,
  o.name1,
  COALESCE(o.name2, '') AS name2,
  COALESCE(sd.name, '') AS triggered_by
FROM icinga_downtimehistory dh USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=dh.object_id
LEFT JOIN icinga_scheduleddowntime sd ON sd.scheduleddowntime_id=dh.triggered_by_id
WHERE dh.downtimehistory_id BETWEEN :fromid AND :toid
AND dh.downtimehistory_id > :checkpoint -- where we were interrupted
ORDER BY dh.downtimehistory_id -- this way we know what has already been migrated from just the last row's ID
LIMIT :bulk
