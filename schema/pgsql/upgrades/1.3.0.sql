ALTER TABLE host ALTER COLUMN icon_image_alt TYPE text;
ALTER TABLE service ALTER COLUMN icon_image_alt TYPE text;

COMMENT ON COLUMN endpoint.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN comment.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN notification.properties_checksum IS 'sha1(all properties)';

ALTER TABLE timeperiod_range ALTER COLUMN range_value TYPE text;

ALTER TABLE checkcommand_argument ALTER COLUMN argument_key TYPE varchar(255);
ALTER TABLE eventcommand_argument ALTER COLUMN argument_key TYPE varchar(255);
ALTER TABLE notificationcommand_argument ALTER COLUMN argument_key TYPE varchar(255);

ALTER TABLE checkcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);
ALTER TABLE eventcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);
ALTER TABLE notificationcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);

CREATE TABLE sla_lifecycle (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  -- These columns are nullable, but as we're using the delete_time to build the composed primary key, we have to set
  -- this to `0` instead, since it's not allowed to use a nullable column as part of the primary key.
  create_time biguint NOT NULL DEFAULT 0,
  delete_time biguint NOT NULL DEFAULT 0,

  CONSTRAINT pk_sla_lifecycle PRIMARY KEY (id, delete_time)
);

COMMENT ON COLUMN sla_lifecycle.id IS 'host.id if service_id is NULL otherwise service.id';
COMMENT ON COLUMN sla_lifecycle.environment_id IS 'environment.id';
COMMENT ON COLUMN sla_lifecycle.host_id IS 'host.id (may reference already deleted hosts)';
COMMENT ON COLUMN sla_lifecycle.service_id IS 'service.id (may reference already deleted services)';
COMMENT ON COLUMN sla_lifecycle.create_time IS 'unix timestamp the event occurred';
COMMENT ON COLUMN sla_lifecycle.delete_time IS 'unix timestamp the delete event occurred';

-- Insert a sla lifecycle create_time entry for all existing hosts with the LEAST timestamp found in either
-- the sla_history_state or sla_history_downtime table, otherwise fallback to the current Unix timestamp.
INSERT INTO sla_lifecycle (id, environment_id, host_id, create_time)
  SELECT host.id,
    host.environment_id,
    host.id,
    COALESCE(LEAST(MIN(event_time), MIN(downtime_start)), EXTRACT(EPOCH FROM now()) * 1000) AS create_time
  FROM host
    LEFT JOIN sla_history_state shs on host.id = shs.host_id AND shs.service_id IS NULL
    LEFT JOIN sla_history_downtime shd on host.id = shd.host_id AND shd.service_id IS NULL
  GROUP BY host.id, host.environment_id
  ON CONFLICT ON CONSTRAINT pk_sla_lifecycle DO NOTHING;

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
  ON CONFLICT ON CONSTRAINT pk_sla_lifecycle DO NOTHING;

-- Insert a sla lifecycle create_time entry for all existing services with the LEAST timestamp found in either
-- the sla_history_state or sla_history_downtime table, otherwise fallback to the current Unix timestamp.
INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, create_time)
  SELECT service.id,
    service.environment_id,
    service.host_id,
    service.id,
    COALESCE(LEAST(MIN(event_time), MIN(downtime_start)), EXTRACT(EPOCH FROM now()) * 1000) AS create_time
  FROM service
    LEFT JOIN sla_history_state shs on service.id = shs.service_id
    LEFT JOIN sla_history_downtime shd on service.id = shd.service_id
  GROUP BY service.id, service.host_id, service.environment_id
  ON CONFLICT ON CONSTRAINT pk_sla_lifecycle DO NOTHING;

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
  ON CONFLICT ON CONSTRAINT pk_sla_lifecycle DO NOTHING;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (4, extract(epoch from now()) * 1000);
