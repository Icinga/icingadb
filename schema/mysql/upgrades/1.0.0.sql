ALTER TABLE hostgroup
    DROP INDEX idx_hostroup_name,
    ADD INDEX idx_hostgroup_name (name) COMMENT 'Host/service/host group list filtered by host group name';

ALTER TABLE notification_history
    MODIFY `text` longtext NOT NULL;

ALTER TABLE host_state
    ADD COLUMN previous_soft_state tinyint unsigned NOT NULL AFTER hard_state;

ALTER TABLE service_state
    ADD COLUMN previous_soft_state tinyint unsigned NOT NULL AFTER hard_state;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (3, CURRENT_TIMESTAMP() * 1000);
