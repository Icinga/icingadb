CREATE TABLE icingadb_config (
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 NOT NULL DEFAULT '\x0000000000000000000000000000000000000000',

  env_key varchar(255) NOT NULL,
  env_value text NOT NULL,

  locked boolenum NOT NULL,

  CONSTRAINT pk_icingadb_config PRIMARY KEY (environment_id, endpoint_id, env_key)
);

ALTER TABLE icingadb_config ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE icingadb_config ALTER COLUMN endpoint_id SET STORAGE PLAIN;

COMMENT ON COLUMN icingadb_config.environment_id IS 'environment.id';
COMMENT ON COLUMN icingadb_config.endpoint_id IS 'endpoint.id, all zero bytes if unknown';
COMMENT ON COLUMN icingadb_config.locked IS 'static config from Icinga DB config is considered as locked';
