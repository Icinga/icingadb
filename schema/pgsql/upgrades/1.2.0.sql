ALTER TABLE customvar_flat ALTER COLUMN flatvalue DROP NOT NULL;

CREATE INDEX idx_customvar_flat_flatname_flatvalue ON customvar_flat(flatname, flatvalue);
COMMENT ON INDEX idx_customvar_flat_flatname_flatvalue IS 'Lists filtered by custom variable';

CREATE INDEX idx_hostgroup_display_name ON hostgroup(display_name);
CREATE INDEX idx_hostgroup_name_ci ON hostgroup(name_ci);
COMMENT ON INDEX idx_hostgroup_display_name IS 'Hostgroup list filtered/ordered by display_name';
COMMENT ON INDEX idx_hostgroup_name_ci IS 'Hostgroup list filtered using quick search';
COMMENT ON INDEX idx_hostgroup_name IS 'Host/service/host group list filtered by host group name; Hostgroup detail filter';

CREATE INDEX idx_servicegroup_display_name ON servicegroup(display_name);
CREATE INDEX idx_servicegroup_name_ci ON servicegroup(name_ci);
COMMENT ON INDEX idx_servicegroup_display_name IS 'Servicegroup list filtered/ordered by display_name';
COMMENT ON INDEX idx_servicegroup_name_ci IS 'Servicegroup list filtered using quick search';
COMMENT ON INDEX idx_servicegroup_name IS 'Host/service/service group list filtered by service group name; Servicegroup detail filter';
