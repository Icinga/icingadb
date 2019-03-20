SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE host_downtime (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  env_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_time bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  duration bigint(20) unsigned NOT NULL,
  is_fixed enum('y', 'n') NOT NULL,

  is_in_effect enum('y', 'n') NOT NULL,
  actual_start_time bigint(20) unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  actual_end_time bigint(20) unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise',

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE service_downtime (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  env_id binary(20) NOT NULL COMMENT 'environment.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_time bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  duration bigint(20) unsigned NOT NULL,
  is_fixed enum('y', 'n') NOT NULL,

  is_in_effect enum('y', 'n') NOT NULL,
  actual_start_time bigint(20) unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  actual_end_time bigint(20) unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise',

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;