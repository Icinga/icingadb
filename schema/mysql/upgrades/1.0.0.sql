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

INSERT INTO icingadb_schema (version, TIMESTAMP)
  VALUES (3, CURRENT_TIMESTAMP() * 1000);
