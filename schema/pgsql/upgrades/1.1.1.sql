ALTER TABLE notification ALTER COLUMN name TYPE varchar(767);
COMMENT ON COLUMN notification.name IS '255+1+255+1+255, i.e. "host.name!service.name!notification.name"';

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

ALTER TYPE history_type RENAME TO history_type_old;
CREATE TYPE history_type AS ENUM ( 'state_change', 'ack_clear', 'downtime_end', 'flapping_end', 'comment_remove', 'comment_add', 'flapping_start', 'downtime_start', 'ack_set', 'notification' );
ALTER TABLE history
  ALTER COLUMN event_type DROP DEFAULT,
  ALTER COLUMN event_type TYPE history_type USING event_type::text::history_type,
  ALTER COLUMN event_type SET DEFAULT 'state_change'::history_type;
DROP TYPE history_type_old;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (2, extract(epoch from now()) * 1000);
