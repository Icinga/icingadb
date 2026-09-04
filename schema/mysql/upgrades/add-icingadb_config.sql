ALTER TABLE icingadb_instance ADD COLUMN notifications_synchronize_with_database enum('n', 'y') NOT NULL;

CREATE TABLE icingadb_config (
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NOT NULL DEFAULT 0x0000000000000000000000000000000000000000 COMMENT 'endpoint.id, all zero bytes if unknown',

  env_key varchar(255) NOT NULL,
  env_value text NOT NULL,

  locked enum('n', 'y') NOT NULL COMMENT 'static config from Icinga DB config is considered as locked',

  CONSTRAINT pk_icingadb_config PRIMARY KEY (environment_id, endpoint_id, env_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
