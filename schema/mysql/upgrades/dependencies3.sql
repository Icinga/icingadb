ALTER TABLE host
    ADD COLUMN affected_children int unsigned DEFAULT NULL AFTER check_retry_interval;

ALTER TABLE host_state
    ADD COLUMN affects_children enum('n', 'y') NOT NULL DEFAULT 'n' AFTER in_downtime;

ALTER TABLE service
    ADD COLUMN affected_children int unsigned DEFAULT NULL AFTER check_retry_interval;

ALTER TABLE service_state
    ADD COLUMN affects_children enum('n', 'y') NOT NULL DEFAULT 'n' AFTER in_downtime;
