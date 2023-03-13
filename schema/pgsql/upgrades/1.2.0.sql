CREATE TABLE sla_lifecycle (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  create_time biguint DEFAULT 0,
  delete_time biguint DEFAULT 0,

  CONSTRAINT pk_sla_lifecycle PRIMARY KEY (id, delete_time)
);

COMMENT ON COLUMN sla_lifecycle.id IS 'sha1(environment.id, host.id, service.id)';
COMMENT ON COLUMN sla_lifecycle.environment_id IS 'environment.id';
COMMENT ON COLUMN sla_lifecycle.host_id IS 'host.id (may reference already deleted hosts)';
COMMENT ON COLUMN sla_lifecycle.service_id IS 'service.id (may reference already deleted services)';
COMMENT ON COLUMN sla_lifecycle.create_time IS 'unix timestamp the event occurred';
COMMENT ON COLUMN sla_lifecycle.delete_time IS 'unix timestamp the delete event occurred';
