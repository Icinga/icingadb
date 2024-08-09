ALTER TABLE host MODIFY COLUMN icon_image_alt TEXT NOT NULL;
ALTER TABLE service MODIFY COLUMN icon_image_alt TEXT NOT NULL;

ALTER TABLE endpoint MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE comment MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE notification MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';

ALTER TABLE timeperiod_range MODIFY COLUMN range_value text NOT NULL;

CREATE TABLE sla_lifecycle (
  id binary(20) NOT NULL COMMENT 'host.id or service.id depending on the checkable type',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id (may reference already deleted hosts)',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id (may reference already deleted services)',

  -- These columns are nullable, but as we're using the delete_time to build the composed primary key, we have to set
  -- this to `0` instead, since it's not allowed to use a nullable column as part of the primary key.
  create_time bigint unsigned NOT NULL DEFAULT 0 COMMENT 'unix timestamp the event occurred',
  delete_time bigint unsigned NOT NULL DEFAULT 0 COMMENT 'unix timestamp the delete event occurred',

  PRIMARY KEY (id, delete_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, create_time, delete_time)
 SELECT id, environment_id, id, NULL, UNIX_TIMESTAMP() * 1000, 0 FROM host ON DUPLICATE KEY UPDATE sla_lifecycle.id = sla_lifecycle.id;

INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, create_time, delete_time)
  SELECT id, environment_id, host_id, id, UNIX_TIMESTAMP() * 1000, 0 FROM service ON DUPLICATE KEY UPDATE sla_lifecycle.id = sla_lifecycle.id;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (6, UNIX_TIMESTAMP() * 1000);
