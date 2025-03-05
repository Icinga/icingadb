SELECT
  ch.commenthistory_id,
  UNIX_TIMESTAMP(ch.entry_time) AS entry_time,
  ch.entry_time_usec,
  ch.entry_type,
  ch.author_name,
  ch.comment_data,
  ch.is_persistent,
  COALESCE(UNIX_TIMESTAMP(ch.expiration_time), 0) AS expiration_time,
  COALESCE(UNIX_TIMESTAMP(ch.deletion_time), 0) AS deletion_time,
  ch.deletion_time_usec,
  COALESCE(ch.name, CONCAT(o.name1, '!', COALESCE(o.name2, ''), '!', ch.commenthistory_id, '-', ch.object_id)) AS name,
  o.objecttype_id, o.name1, COALESCE(o.name2, '') AS name2
FROM icinga_commenthistory ch USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=ch.object_id
WHERE ch.commenthistory_id BETWEEN :fromid AND :toid
AND ch.commenthistory_id > :checkpoint -- where we were interrupted
ORDER BY ch.commenthistory_id -- this way we know what has already been migrated from just the last row's ID
LIMIT :bulk
