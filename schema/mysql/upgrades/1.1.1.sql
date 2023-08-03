ALTER TABLE notification
    MODIFY COLUMN name varchar(767) NOT NULL COMMENT '255+1+255+1+255, i.e. "host.name!service.name!notification.name"',
    MODIFY COLUMN name_ci varchar(767) COLLATE utf8mb4_unicode_ci NOT NULL;

ALTER TABLE customvar_flat MODIFY COLUMN flatvalue text DEFAULT NULL;

ALTER TABLE customvar_flat
    ADD INDEX idx_customvar_flat_flatname_flatvalue (flatname, flatvalue(255)) COMMENT 'Lists filtered by custom variable';

ALTER TABLE hostgroup
    ADD INDEX idx_hostgroup_display_name (display_name) COMMENT 'Hostgroup list filtered/ordered by display_name',
    ADD INDEX idx_hostgroup_name_ci (name_ci) COMMENT 'Hostgroup list filtered using quick search',
    DROP INDEX idx_hostgroup_name,
    ADD INDEX idx_hostgroup_name (name) COMMENT 'Host/service/host group list filtered by host group name; Hostgroup detail filter';

ALTER TABLE servicegroup
    ADD INDEX idx_servicegroup_display_name (display_name) COMMENT 'Servicegroup list filtered/ordered by display_name',
    ADD INDEX idx_servicegroup_name_ci (name_ci) COMMENT 'Servicegroup list filtered using quick search',
    DROP INDEX idx_servicegroup_name,
    ADD INDEX idx_servicegroup_name (name) COMMENT 'Host/service/service group list filtered by service group name; Servicegroup detail filter';

-- The following sequence of statements changes the type of history.event_type like the following statement would:
--
--   ALTER TABLE history MODIFY event_type enum('state_change', 'ack_clear', 'downtime_end', 'flapping_end', 'comment_remove', 'comment_add', 'flapping_start', 'downtime_start', 'ack_set', 'notification') NOT NULL;
--
-- It's just much faster to add a second column, copy the column using an UPDATE statement and then replace the
-- old column with the one just generated. Table locking ensures that no other connection inserts data in the meantime.
LOCK TABLES history WRITE;
ALTER TABLE history ADD COLUMN event_type_new enum('state_change', 'ack_clear', 'downtime_end', 'flapping_end', 'comment_remove', 'comment_add', 'flapping_start', 'downtime_start', 'ack_set', 'notification') NOT NULL AFTER event_type;
UPDATE history SET event_type_new = event_type;
ALTER TABLE history
    DROP COLUMN event_type,
    CHANGE COLUMN event_type_new event_type enum('state_change', 'ack_clear', 'downtime_end', 'flapping_end', 'comment_remove', 'comment_add', 'flapping_start', 'downtime_start', 'ack_set', 'notification') NOT NULL;
UNLOCK TABLES;
