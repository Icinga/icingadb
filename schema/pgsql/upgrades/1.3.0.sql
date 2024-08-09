ALTER TABLE host ALTER COLUMN icon_image_alt TYPE text;
ALTER TABLE service ALTER COLUMN icon_image_alt TYPE text;

COMMENT ON COLUMN endpoint.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN comment.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN notification.properties_checksum IS 'sha1(all properties)';

ALTER TABLE timeperiod_range ALTER COLUMN range_value TYPE text;

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

COMMENT ON COLUMN sla_lifecycle.id IS 'host.id or service.id depending on the checkable type';
COMMENT ON COLUMN sla_lifecycle.environment_id IS 'environment.id';
COMMENT ON COLUMN sla_lifecycle.host_id IS 'host.id (may reference already deleted hosts)';
COMMENT ON COLUMN sla_lifecycle.service_id IS 'service.id (may reference already deleted services)';
COMMENT ON COLUMN sla_lifecycle.create_time IS 'unix timestamp the event occurred';
COMMENT ON COLUMN sla_lifecycle.delete_time IS 'unix timestamp the delete event occurred';

INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, create_time, delete_time)
  SELECT id, environment_id, id, NULL, EXTRACT(EPOCH FROM now()) * 1000, 0 FROM host ON CONFLICT ON CONSTRAINT pk_sla_lifecycle DO NOTHING;

INSERT INTO sla_lifecycle (id, environment_id, host_id, service_id, create_time, delete_time)
  SELECT id, environment_id, host_id, id, EXTRACT(EPOCH FROM now()) * 1000, 0 FROM service ON CONFLICT ON CONSTRAINT pk_sla_lifecycle DO NOTHING;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (4, extract(epoch from now()) * 1000);
