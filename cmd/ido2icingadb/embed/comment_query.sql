SELECT ch.commenthistory_id, UNIX_TIMESTAMP(ch.entry_time) entry_time,
  ch.entry_time_usec, ch.entry_type, ch.author_name, ch.comment_data, ch.is_persistent,
  COALESCE(UNIX_TIMESTAMP(ch.expiration_time), 0) expiration_time,
  COALESCE(UNIX_TIMESTAMP(ch.deletion_time), 0) deletion_time,
  ch.deletion_time_usec, ch.name, o.objecttype_id, o.name1, COALESCE(o.name2, '') name2
FROM icinga_commenthistory ch USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=ch.object_id
WHERE ch.commenthistory_id > :checkpoint -- where we were interrupted
ORDER BY ch.commenthistory_id -- this way we know what has already been migrated from just the last row's ID
LIMIT :bulk
