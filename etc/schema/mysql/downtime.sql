SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE downtime (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  triggered_by_id binary(20) NULL DEFAULT NULL COMMENT 'downtime.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_time bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  flexible_duration bigint(20) unsigned NOT NULL,
  is_flexible enum('y', 'n') NOT NULL,

  is_in_effect enum('y', 'n') NOT NULL,
  start_time bigint(20) unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  end_time bigint(20) unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + flexible_duration otherwise',

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;