SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE comment (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  text text NOT NULL,
  entry_type enum('comment','ack','downtime','flapping') NOT NULL,
  entry_time bigint(20) unsigned NOT NULL,
  is_persistent enum('y','n') NOT NULL,
  is_sticky enum('y','n') NOT NULL,
  expire_time bigint(20) unsigned DEFAULT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;