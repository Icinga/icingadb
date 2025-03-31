SELECT
  n.notification_id,
  n.notification_reason,
  UNIX_TIMESTAMP(n.end_time) AS end_time,
  n.end_time_usec,
  n.state,
  COALESCE(n.output, '') AS output,
  n.long_output,
  n.contacts_notified,
  o.objecttype_id,
  o.name1,
  COALESCE(o.name2, '') AS name2
FROM icinga_notifications n USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=n.object_id
WHERE n.notification_id BETWEEN :fromid AND :toid
AND n.notification_id <= :cache_limit AND n.notification_id > :checkpoint -- where we were interrupted
ORDER BY n.notification_id -- this way we know what has already been migrated from just the last row's ID
LIMIT :bulk
