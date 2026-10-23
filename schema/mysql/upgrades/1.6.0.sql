-- schema: Index user_notification_history.notification_history_id

ALTER TABLE user_notification_history ADD INDEX idx_user_notification_history_notification_history_id (notification_history_id) COMMENT 'Speed up ON DELETE CASCADE';

-- Align Icinga 2 Types with SQL Representation

ALTER TABLE host
  MODIFY COLUMN check_timeout float DEFAULT NULL,
  MODIFY COLUMN check_interval float NOT NULL,
  MODIFY COLUMN check_retry_interval float NOT NULL;

ALTER TABLE host_state
  MODIFY COLUMN execution_time float DEFAULT NULL,
  MODIFY COLUMN latency float DEFAULT NULL,
  MODIFY COLUMN check_timeout float DEFAULT NULL;

ALTER TABLE service
  MODIFY COLUMN check_timeout float DEFAULT NULL,
  MODIFY COLUMN check_interval float NOT NULL,
  MODIFY COLUMN check_retry_interval float NOT NULL;

ALTER TABLE service_state
  MODIFY COLUMN execution_time float DEFAULT NULL,
  MODIFY COLUMN latency float DEFAULT NULL,
  MODIFY COLUMN check_timeout float DEFAULT NULL;

-- Probe notifications socket & monitor API health

ALTER TABLE icingadb_instance
  ADD COLUMN icingadb_service_user varchar(255) NOT NULL DEFAULT 'icingadb',
  ADD COLUMN notifications_healthy enum('n', 'y') DEFAULT NULL,
  ADD COLUMN notifications_discovered_socket_path mediumtext;

ALTER TABLE icingadb_instance MODIFY COLUMN icingadb_service_user varchar(255) NOT NULL;

-- Notifications: Load Configuration from Database

ALTER TABLE icingadb_instance ADD COLUMN notifications_synchronize_with_database enum('n', 'y') NOT NULL;

CREATE TABLE icingadb_config (
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NOT NULL DEFAULT 0x0000000000000000000000000000000000000000 COMMENT 'endpoint.id, all zero bytes if unknown',

  env_key varchar(255) NOT NULL,
  env_value text NOT NULL,

  locked enum('n', 'y') NOT NULL COMMENT 'static config from Icinga DB config is considered as locked',

  CONSTRAINT pk_icingadb_config PRIMARY KEY (environment_id, endpoint_id, env_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

-- Correlate Icinga Notifications history with our history table

ALTER TABLE host_state ADD COLUMN acknowledgement_set_time bigint unsigned DEFAULT NULL AFTER is_sticky_acknowledgement;
ALTER TABLE service_state ADD COLUMN acknowledgement_set_time bigint unsigned DEFAULT NULL AFTER is_sticky_acknowledgement;

ALTER TABLE history ADD COLUMN alert_count bigint unsigned DEFAULT NULL;

-- alert_history_ref contains a reference to the history.id of rows that are of interest to the alert_history table.
-- When Icinga DB regularly runs its background alert history correlation job, it will use this table to find the
-- matching history.id rows based on the history_id_uuid. Once a particular incident has been closed, all the matching
-- rows pertaining to that incident will be deleted from this table.
CREATE TABLE alert_history_ref (
  history_id binary(20) NOT NULL, -- history.id. No foreign key constraint, as the ref may be written before the history row.
  history_id_uuid binary(16) NOT NULL, -- uuid of the history.id, used for event correlation with the alert_history table.
  host_id binary(20) NOT NULL, -- host.id
  service_id binary(20) DEFAULT NULL, -- service.id

  last_update bigint unsigned NOT NULL, -- unix timestamp of the last insert or update of this row.

  PRIMARY KEY (history_id),

  UNIQUE INDEX uq_alert_history_ref_history_id_uuid (history_id_uuid),
  INDEX idx_alert_history_ref_host_id_service_id (host_id, service_id),
  -- The following index is used to find all alert_history_ref rows for a given service_id, which is used by
  -- the cleanup query in Client.ApplyDelta to delete all alert_history_ref rows for a given service_id when
  -- the service is deleted.
  INDEX idx_alert_history_ref_service_id (service_id),
  INDEX idx_alert_history_ref_last_update (last_update)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE alert_history (
  id binary(20) NOT NULL, -- sha1(history_id + contact_name + contactgroup_name + schedule_name + channel_name + triggered_at)
  history_id binary(20) NOT NULL, -- history.id
  environment_id binary(20) NOT NULL, -- environment.id
  contact_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  contactgroup_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  schedule_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  channel_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  triggered_at bigint unsigned NOT NULL,
  event_message longtext NOT NULL,

  PRIMARY KEY (id),

  CONSTRAINT fk_alert_history_history FOREIGN KEY (history_id) REFERENCES history (id) ON DELETE CASCADE,

  INDEX idx_alert_history_environment_id_triggered_at (environment_id, triggered_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

-- notifications: populate `alert_history_ref` for downtime and flapping events

ALTER TABLE alert_history_ref DROP PRIMARY KEY;
ALTER TABLE alert_history_ref
  ADD PRIMARY KEY (history_id_uuid, history_id);

-- Include `event_summary` in the `alert_history` table

ALTER TABLE alert_history ADD COLUMN event_summary longtext;
UPDATE alert_history SET event_summary = COALESCE(NULLIF(LEFT(TRIM(SUBSTRING_INDEX(event_message, '\n', 1)), 1024), ''), 'no summary');
ALTER TABLE alert_history MODIFY COLUMN event_summary longtext NOT NULL;

-- Bump schema

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (8, UNIX_TIMESTAMP() * 1000);
