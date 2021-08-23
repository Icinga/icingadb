ALTER TABLE hostgroup
    DROP INDEX idx_hostroup_name,
    ADD INDEX idx_hostgroup_name (name) COMMENT 'Host/service/host group list filtered by host group name';

ALTER TABLE notification_history
  MODIFY `text` longtext NOT NULL;

ALTER TABLE host_state
  ADD COLUMN previous_soft_state tinyint unsigned NOT NULL AFTER hard_state;

ALTER TABLE service_state
  ADD COLUMN previous_soft_state tinyint unsigned NOT NULL AFTER hard_state;

ALTER TABLE acknowledgement_history
  ADD index idx_acknowledgement_history_clear_time (clear_time) COMMENT 'Filter for history retention';

ALTER TABLE comment_history
  ADD index idx_comment_history_remove_time (remove_time) COMMENT 'Filter for history retention';

ALTER TABLE downtime_history
  ADD index idx_downtime_history_end_time (end_time) COMMENT 'Filter for history retention';

ALTER TABLE flapping_history
  ADD index idx_flapping_history_end_time (end_time) COMMENT 'Filter for history retention';

ALTER TABLE state_history
  ADD index idx_state_history_event_time (event_time) COMMENT 'Filter for history retention';

ALTER TABLE icon_image
  DROP PRIMARY KEY,
  MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + icon_image)',
  ADD PRIMARY KEY (id);

ALTER TABLE action_url
  DROP PRIMARY KEY,
  MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + action_url)',
  ADD PRIMARY KEY (id);

ALTER TABLE notes_url
  DROP PRIMARY KEY,
  MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notes_url)',
  ADD PRIMARY KEY (id);

ALTER TABLE customvar
  MODIFY name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL;

ALTER TABLE customvar_flat
  MODIFY flatname varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Path converted with `.` and `[ ]`';

CREATE TABLE sla_history_state (
  id binary(20) NOT NULL COMMENT 'state_history.id (may reference already deleted rows)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  event_time bigint unsigned NOT NULL COMMENT 'unix timestamp the event occurred',
  hard_state TINYINT UNSIGNED NOT NULL COMMENT 'hard state after this event',
  previous_hard_state TINYINT UNSIGNED NOT NULL COMMENT 'hard state before this event',

  PRIMARY KEY (id),

  INDEX idx_sla_history_state_event (host_id, service_id, event_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE sla_history_downtime (
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  downtime_id binary(20) NOT NULL COMMENT 'downtime.id (may reference already deleted rows)',
  downtime_start BIGINT UNSIGNED NOT NULL COMMENT 'start time of the downtime',
  downtime_end BIGINT UNSIGNED NOT NULL COMMENT 'end time of the downtime',

  PRIMARY KEY (downtime_id),

  INDEX idx_sla_history_downtime_event (host_id, service_id, downtime_start, downtime_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

INSERT INTO icingadb_schema (version, TIMESTAMP)
  VALUES (3, CURRENT_TIMESTAMP() * 1000);
