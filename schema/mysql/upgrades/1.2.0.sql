CREATE TABLE sla_lifecycle (
  id binary(20) NOT NULL COMMENT 'sha1(environment.id, host.id, service.id)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id (may reference already deleted hosts)',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id (may reference already deleted services)',

  -- These columns are nullable, but as we're using the delete_time to build the composed primary key, we have to set
  --  this to `0` instead, since it's not allowed to use a nullable column as part of the primary key.
  create_time bigint unsigned NOT NULL DEFAULT 0 COMMENT 'unix timestamp the event occurred',
  delete_time bigint unsigned NOT NULL DEFAULT 0 COMMENT 'unix timestamp the delete event occurred',

  PRIMARY KEY (id, delete_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
