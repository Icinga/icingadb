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
