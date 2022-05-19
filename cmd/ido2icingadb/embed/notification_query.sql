SELECT n.notification_id, n.notification_reason, UNIX_TIMESTAMP(n.end_time) end_time,
  n.end_time_usec, n.state, COALESCE(n.output, '') output, n.long_output,
  n.contacts_notified, o.objecttype_id, o.name1, COALESCE(o.name2, '') name2
FROM icinga_notifications n USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=n.object_id
WHERE n.notification_id <= :cache_limit AND n.notification_id > :checkpoint -- where we were interrupted
ORDER BY n.notification_id -- allows computeProgress() not to check all IDO rows for whether migrated
LIMIT :bulk
