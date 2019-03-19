SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE environment (
  id binary(20) NOT NULL COMMENT 'sha1(name)',
  name varchar(255) NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE icingadb_instance (
  id binary(16) NOT NULL COMMENT 'UUIDv4',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  heartbeat bigint(20) unsigned NOT NULL COMMENT '*nix timestamp',
  responsible enum('y','n') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;