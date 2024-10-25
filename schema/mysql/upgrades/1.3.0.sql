ALTER TABLE host MODIFY COLUMN icon_image_alt TEXT NOT NULL;
ALTER TABLE service MODIFY COLUMN icon_image_alt TEXT NOT NULL;

ALTER TABLE endpoint MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE comment MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE notification MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';

ALTER TABLE timeperiod_range MODIFY COLUMN range_value text NOT NULL;

ALTER TABLE checkcommand_argument MODIFY COLUMN argument_key varchar(255) NOT NULL;
ALTER TABLE checkcommand_argument MODIFY COLUMN argument_key_override varchar(255) NOT NULL;
ALTER TABLE eventcommand_argument MODIFY COLUMN argument_key varchar(255) NOT NULL;
ALTER TABLE eventcommand_argument MODIFY COLUMN argument_key_override varchar(255) NOT NULL;
ALTER TABLE notificationcommand_argument MODIFY COLUMN argument_key varchar(255) NOT NULL;
ALTER TABLE notificationcommand_argument MODIFY COLUMN argument_key_override varchar(255) NOT NULL;

ALTER TABLE checkcommand_envvar MODIFY COLUMN envvar_key varchar(255) NOT NULL;
ALTER TABLE eventcommand_envvar MODIFY COLUMN envvar_key varchar(255) NOT NULL;
ALTER TABLE notificationcommand_envvar MODIFY COLUMN envvar_key varchar(255) NOT NULL;

CREATE TABLE sla_lifecycle (
  id binary(20) NOT NULL COMMENT 'host.id if service_id is NULL otherwise service.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id (may reference already deleted hosts)',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id (may reference already deleted services)',

  -- These columns are nullable, but as we're using the delete_time to build the composed primary key, we have to set
  -- this to `0` instead, since it's not allowed to use a nullable column as part of the primary key.
  create_time bigint unsigned NOT NULL DEFAULT 0 COMMENT 'unix timestamp the event occurred',
  delete_time bigint unsigned NOT NULL DEFAULT 0 COMMENT 'unix timestamp the delete event occurred',

  PRIMARY KEY (id, delete_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

-- Insert a sla lifecycle create_time entry for all existing hosts with the LEAST timestamp found in either
-- the sla_history_state or sla_history_downtime table, otherwise fallback to the current Unix timestamp.
INSERT INTO sla_lifecycle (id, environment_id, host_id, create_time)
  SELECT host.id,
    host.environment_id,
    host.id,
    -- In MySQL/MariaDB, LEAST() returns NULL if either event_time or downtime_start is NULL, which is not
    -- desirable for our use cases. So we need to work around this behaviour by nesting some COALESCE() calls.
    COALESCE(LEAST(COALESCE(MIN(event_time), MIN(downtime_start)), COALESCE(MIN(downtime_start), MIN(event_time))), UNIX_TIMESTAMP() * 1000) AS create_time
  FROM host
    LEFT JOIN sla_history_state shs on host.id = shs.host_id AND shs.service_id IS NULL
    LEFT JOIN sla_history_downtime shd on host.id = shd.host_id AND shd.service_id IS NULL
  GROUP BY host.id, host.environment_id
  ON DUPLICATE KEY UPDATE sla_lifecycle.id = sla_lifecycle.id;

-- Insert a sla lifecycle deleted entry for all not existing hosts with the GREATEST timestamp
-- found in either the sla_history_state or sla_history_downtime table.
INSERT INTO sla_lifecycle (id, environment_id, host_id, delete_time)
  SELECT host_id AS id,
    environment_id,
    host_id,
    MAX(event_time) AS delete_time
  FROM (SELECT host_id, environment_id, MAX(event_time) AS event_time
    FROM sla_history_state
      WHERE service_id IS NULL AND NOT EXISTS(SELECT 1 FROM host WHERE id = host_id)
    GROUP BY host_id, environment_id
    UNION ALL
    SELECT host_id, environment_id, MAX(downtime_end) AS event_time
    FROM sla_history_downtime
      WHERE service_id IS NULL AND NOT EXISTS(SELECT 1 FROM host WHERE id = host_id)
    GROUP BY host_id, environment_id
  ) AS deleted_hosts
  GROUP BY host_id, environment_id HAVING MAX(event_time) IS NOT NULL
  ON DUPLICATE KEY UPDATE sla_lifecycle.id = sla_lifecycle.id;

-- Insert a sla lifecycle create_time entry for all existing services with the LEAST timestamp found in either
-- the sla_history_state or sla_history_downtime table, otherwise fallback to the current Unix timestamp.
INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, create_time)
  SELECT service.id,
    service.environment_id,
    service.host_id,
    service.id,
     -- In MySQL/MariaDB, LEAST() returns NULL if either event_time or downtime_start is NULL, which is not
     -- desirable for our use cases. So we need to work around this behaviour by nesting some COALESCE() calls.
    COALESCE(LEAST(COALESCE(MIN(event_time), MIN(downtime_start)), COALESCE(MIN(downtime_start), MIN(event_time))), UNIX_TIMESTAMP() * 1000) AS create_time
  FROM service
    LEFT JOIN sla_history_state shs on service.id = shs.service_id
    LEFT JOIN sla_history_downtime shd on service.id = shd.service_id
  GROUP BY service.id, service.host_id, service.environment_id
  ON DUPLICATE KEY UPDATE sla_lifecycle.id = sla_lifecycle.id;

-- Insert a sla lifecycle deleted entry for all not existing hosts with the GREATEST timestamp
-- found in either the sla_history_state or sla_history_downtime table.
INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, delete_time)
  SELECT service_id AS id,
    environment_id,
    host_id,
    service_id,
    MAX(event_time) AS delete_time
  FROM (SELECT service_id, environment_id, host_id, MAX(event_time) AS event_time
    FROM sla_history_state
      WHERE service_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM service WHERE id = service_id)
    GROUP BY service_id, environment_id, host_id
    UNION ALL
    SELECT service_id, environment_id, host_id, MAX(downtime_end) AS event_time
    FROM sla_history_downtime
      WHERE service_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM service WHERE id = service_id)
    GROUP BY service_id, environment_id, host_id
  ) AS deleted_services
  GROUP BY service_id, environment_id, host_id HAVING MAX(event_time) IS NOT NULL
  ON DUPLICATE KEY UPDATE sla_lifecycle.id = sla_lifecycle.id;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (6, UNIX_TIMESTAMP() * 1000);
