ALTER TABLE hostgroup
    DROP INDEX idx_hostroup_name,
    ADD INDEX idx_hostgroup_name (name) COMMENT 'Host/service/host group list filtered by host group name';

ALTER TABLE notification_history
    MODIFY `text` longtext NOT NULL;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (3, CURRENT_TIMESTAMP() * 1000);
