SELECT ah.acknowledgement_id, UNIX_TIMESTAMP(ah.entry_time) entry_time, ah.entry_time_usec,
  ah.acknowledgement_type, ah.author_name, ah.comment_data, ah.is_sticky, ah.persistent_comment,
  UNIX_TIMESTAMP(ah.end_time) end_time, o.objecttype_id, o.name1, COALESCE(o.name2, '') name2
FROM icinga_acknowledgements ah USE INDEX (PRIMARY)
INNER JOIN icinga_objects o ON o.object_id=ah.object_id
WHERE ah.acknowledgement_id > :checkpoint -- where we were interrupted
ORDER BY ah.acknowledgement_id -- this way we know what has already been migrated from just the last row's ID
LIMIT :bulk
