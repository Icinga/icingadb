SELECT dh.downtimehistory_id, UNIX_TIMESTAMP(dh.entry_time) entry_time, dh.author_name, dh.comment_data,
  dh.is_fixed, dh.duration, UNIX_TIMESTAMP(dh.scheduled_start_time) scheduled_start_time,
  COALESCE(UNIX_TIMESTAMP(dh.scheduled_end_time), 0) scheduled_end_time,
  COALESCE(UNIX_TIMESTAMP(dh.actual_start_time), 0) actual_start_time, dh.actual_start_time_usec,
  COALESCE(UNIX_TIMESTAMP(dh.actual_end_time), 0) actual_end_time, dh.actual_end_time_usec, dh.was_cancelled,
  COALESCE(UNIX_TIMESTAMP(dh.trigger_time), 0) trigger_time, dh.name, o.objecttype_id,
  o.name1, COALESCE(o.name2, '') name2, COALESCE(sd.name, '') triggered_by
FROM icinga_downtimehistory dh USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=dh.object_id
LEFT JOIN icinga_scheduleddowntime sd ON sd.scheduleddowntime_id=dh.triggered_by_id
WHERE dh.downtimehistory_id > :checkpoint -- where we were interrupted
ORDER BY dh.downtimehistory_id -- allows computeProgress() not to check all IDO rows for whether migrated
LIMIT :bulk
