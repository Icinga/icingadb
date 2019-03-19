SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE endpoint (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  env_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) NOT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;