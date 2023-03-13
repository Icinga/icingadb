CREATE TABLE sla_lifecycle (
  id binary(20) NOT NULL COMMENT 'sha1(environment.id, host.id, service.id)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id (may reference already deleted hosts)',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id (may reference already deleted services)',

  create_time bigint unsigned DEFAULT 0 COMMENT 'unix timestamp the event occurred',
  delete_time bigint unsigned DEFAULT 0 COMMENT 'unix timestamp the delete event occurred',

  PRIMARY KEY (id, delete_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
