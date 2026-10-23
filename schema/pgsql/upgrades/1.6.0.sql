-- schema: Index user_notification_history.notification_history_id

CREATE INDEX idx_user_notification_history_notification_history_id ON user_notification_history(notification_history_id);
COMMENT ON INDEX idx_user_notification_history_notification_history_id IS 'Speed up ON DELETE CASCADE';

-- Align Icinga 2 Types with SQL Representation

ALTER TABLE host
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL,
  ALTER COLUMN check_interval TYPE float,
  ALTER COLUMN check_retry_interval TYPE float;

ALTER TABLE host_state
  ALTER COLUMN execution_time TYPE float,
  ALTER COLUMN execution_time SET DEFAULT NULL,
  ALTER COLUMN latency TYPE float,
  ALTER COLUMN latency SET DEFAULT NULL,
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL;

ALTER TABLE service
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL,
  ALTER COLUMN check_interval TYPE float,
  ALTER COLUMN check_retry_interval TYPE float;

ALTER TABLE service_state
  ALTER COLUMN execution_time TYPE float,
  ALTER COLUMN execution_time SET DEFAULT NULL,
  ALTER COLUMN latency TYPE float,
  ALTER COLUMN latency SET DEFAULT NULL,
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL;

-- Probe notifications socket & monitor API health

ALTER TABLE icingadb_instance
  ADD COLUMN icingadb_service_user varchar(255) NOT NULL DEFAULT 'icingadb',
  ADD COLUMN notifications_healthy boolenum DEFAULT NULL,
  ADD COLUMN notifications_discovered_socket_path text;

ALTER TABLE icingadb_instance ALTER COLUMN icingadb_service_user DROP DEFAULT;

-- Notifications: Load Configuration from Database

ALTER TABLE icingadb_instance ADD COLUMN notifications_synchronize_with_database boolenum NOT NULL DEFAULT 'n';

CREATE TABLE icingadb_config (
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 NOT NULL DEFAULT '\x0000000000000000000000000000000000000000',

  env_key varchar(255) NOT NULL,
  env_value text NOT NULL,

  locked boolenum NOT NULL DEFAULT 'n',

  CONSTRAINT pk_icingadb_config PRIMARY KEY (environment_id, endpoint_id, env_key)
);

ALTER TABLE icingadb_config ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE icingadb_config ALTER COLUMN endpoint_id SET STORAGE PLAIN;

COMMENT ON COLUMN icingadb_config.environment_id IS 'environment.id';
COMMENT ON COLUMN icingadb_config.endpoint_id IS 'endpoint.id, all zero bytes if unknown';
COMMENT ON COLUMN icingadb_config.locked IS 'static config from Icinga DB config is considered as locked';

-- Correlate Icinga Notifications history with our history table

ALTER TABLE host_state ADD COLUMN acknowledgement_set_time biguint DEFAULT NULL;
ALTER TABLE service_state ADD COLUMN acknowledgement_set_time biguint DEFAULT NULL;

ALTER TABLE history ADD COLUMN alert_count biguint DEFAULT NULL;

-- alert_history_ref contains a reference to the history.id of rows that are of interest to the alert_history table.
-- When Icinga DB regularly runs its background alert history correlation job, it will use this table to find the
-- matching history.id rows based on the history_id_uuid. Once a particular incident has been closed, all the matching
-- rows pertaining to that incident will be deleted from this table.
CREATE TABLE alert_history_ref (
  history_id bytea20 NOT NULL, -- history.id. No foreign key constraint, as the ref may be written before the history row.
  history_id_uuid uuid NOT NULL, -- uuid of the history.id, used for event correlation with the alert_history table.
  host_id bytea20 NOT NULL, -- host.id
  service_id bytea20 DEFAULT NULL, -- service.id

  last_update biguint NOT NULL, -- unix timestamp of the last insert or update of this row.

  CONSTRAINT pk_alert_history_ref PRIMARY KEY (history_id),
  CONSTRAINT uq_alert_history_ref_history_id_uuid UNIQUE (history_id_uuid)
);

CREATE INDEX idx_alert_history_ref_host_id_service_id ON alert_history_ref(host_id, service_id);
-- The following index is used to find all alert_history_ref rows for a given service_id, which is used by
-- the cleanup query in Client.ApplyDelta to delete all alert_history_ref rows for a given service_id when
-- the service is deleted.
CREATE INDEX idx_alert_history_ref_service_id ON alert_history_ref (service_id);
CREATE INDEX idx_alert_history_ref_last_update ON alert_history_ref(last_update);

ALTER TABLE alert_history_ref ALTER COLUMN history_id SET STORAGE PLAIN;
ALTER TABLE alert_history_ref ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE alert_history_ref ALTER COLUMN service_id SET STORAGE PLAIN;

CREATE TABLE alert_history (
  id bytea20 NOT NULL, -- sha1(history_id + contact_name + contactgroup_name + schedule_name + channel_name + triggered_at)
  history_id bytea20 NOT NULL, -- history.id
  environment_id bytea20 NOT NULL, -- environment.id
  contact_name citext,
  contactgroup_name citext,
  schedule_name citext,
  channel_name citext,
  triggered_at biguint NOT NULL,
  event_message text NOT NULL,

  CONSTRAINT pk_alert_history PRIMARY KEY (id),

  CONSTRAINT fk_alert_history_history FOREIGN KEY (history_id) REFERENCES history (id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_history_history_id ON alert_history(history_id);
CREATE INDEX idx_alert_history_environment_id_triggered_at ON alert_history (environment_id, triggered_at);

ALTER TABLE alert_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE alert_history ALTER COLUMN history_id SET STORAGE PLAIN;
ALTER TABLE alert_history ALTER COLUMN environment_id SET STORAGE PLAIN;

-- notifications: populate `alert_history_ref` for downtime and flapping events

ALTER TABLE alert_history_ref
  DROP CONSTRAINT pk_alert_history_ref;
ALTER TABLE alert_history_ref
  ADD CONSTRAINT pk_alert_history_ref PRIMARY KEY (history_id_uuid, history_id);

-- Include `event_summary` in the `alert_history` table

ALTER TABLE alert_history ADD COLUMN event_summary text;
UPDATE alert_history SET event_summary = COALESCE(NULLIF(LEFT(BTRIM(SPLIT_PART(event_message, E'\n', 1)), 1024), ''), 'no summary');
ALTER TABLE alert_history ALTER COLUMN event_summary SET NOT NULL;

-- Bump schema

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (6, extract(epoch from now()) * 1000);
