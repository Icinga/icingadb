-- Icinga DB | (c) 2021 Icinga GmbH | GPLv2+

-- Postgres in Docker: ensure CITEXT columns are available during schema import. DB user is a superuser and can do this unconditionally.
-- Everything else: assert CITEXT columns are available during schema import. DB user isn't the superuser and can do this only if it's a no-op (`NOTICE:  extension "citext" already exists, skipping`), i.e. if CITEXT columns are already available.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE DOMAIN bytea20 AS bytea CONSTRAINT exactly_20_bytes_long CHECK ( VALUE IS NULL OR octet_length(VALUE) = 20 );
CREATE DOMAIN bytea16 AS bytea CONSTRAINT exactly_16_bytes_long CHECK ( VALUE IS NULL OR octet_length(VALUE) = 16 );
CREATE DOMAIN bytea4 AS bytea CONSTRAINT exactly_4_bytes_long CHECK ( VALUE IS NULL OR octet_length(VALUE) = 4 );

CREATE DOMAIN biguint AS bigint CONSTRAINT positive CHECK ( VALUE IS NULL OR 0 <= VALUE );
CREATE DOMAIN uint AS bigint CONSTRAINT between_0_and_4294967295 CHECK ( VALUE IS NULL OR VALUE BETWEEN 0 AND 4294967295 );
CREATE DOMAIN smalluint AS int CONSTRAINT between_0_and_65535 CHECK ( VALUE IS NULL OR VALUE BETWEEN 0 AND 65535 );
CREATE DOMAIN tinyuint AS smallint CONSTRAINT between_0_and_255 CHECK ( VALUE IS NULL OR VALUE BETWEEN 0 AND 255 );

CREATE TYPE boolenum AS ENUM ( 'n', 'y' );
CREATE TYPE acked AS ENUM ( 'n', 'y', 'sticky' );
CREATE TYPE state_type AS ENUM ( 'hard', 'soft' );
CREATE TYPE checkable_type AS ENUM ( 'host', 'service' );
CREATE TYPE comment_type AS ENUM ( 'comment', 'ack' );
CREATE TYPE notification_type AS ENUM ( 'downtime_start', 'downtime_end', 'downtime_removed', 'custom', 'acknowledgement', 'problem', 'recovery', 'flapping_start', 'flapping_end' );
CREATE TYPE history_type AS ENUM ( 'notification', 'state_change', 'downtime_start', 'downtime_end', 'comment_add', 'comment_remove', 'flapping_start', 'flapping_end', 'ack_set', 'ack_clear' );

CREATE TABLE host (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,

  address varchar(255) NOT NULL,
  address6 varchar(255) NOT NULL,
  address_bin bytea4 DEFAULT NULL,
  address6_bin bytea16 DEFAULT NULL,

  checkcommand citext NOT NULL,
  checkcommand_id bytea20 NOT NULL,

  max_check_attempts uint NOT NULL,

  check_timeperiod citext NOT NULL,
  check_timeperiod_id bytea20 DEFAULT NULL,

  check_timeout uint DEFAULT NULL,
  check_interval uint NOT NULL,
  check_retry_interval uint NOT NULL,

  active_checks_enabled boolenum NOT NULL DEFAULT 'n',
  passive_checks_enabled boolenum NOT NULL DEFAULT 'n',
  event_handler_enabled boolenum NOT NULL DEFAULT 'n',
  notifications_enabled boolenum NOT NULL DEFAULT 'n',

  flapping_enabled boolenum NOT NULL DEFAULT 'n',
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  perfdata_enabled boolenum NOT NULL DEFAULT 'n',

  eventcommand citext NOT NULL,
  eventcommand_id bytea20 DEFAULT NULL,

  is_volatile boolenum NOT NULL DEFAULT 'n',

  action_url_id bytea20 DEFAULT NULL,
  notes_url_id bytea20 DEFAULT NULL,
  notes text NOT NULL,
  icon_image_id bytea20 DEFAULT NULL,
  icon_image_alt varchar(32) NOT NULL,

  zone citext NOT NULL,
  zone_id bytea20 DEFAULT NULL,

  command_endpoint citext NOT NULL,
  command_endpoint_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_host PRIMARY KEY (id)
);

ALTER TABLE host ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN address_bin SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN address6_bin SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN checkcommand_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN check_timeperiod_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN eventcommand_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN action_url_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN notes_url_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN icon_image_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE host ALTER COLUMN command_endpoint_id SET STORAGE PLAIN;

CREATE INDEX idx_action_url_checksum ON host(action_url_id);
CREATE INDEX idx_notes_url_checksum ON host(notes_url_id);
CREATE INDEX idx_icon_image_checksum ON host(icon_image_id);
CREATE INDEX idx_host_display_name ON host(display_name);
CREATE INDEX idx_host_name_ci ON host(name_ci);
CREATE INDEX idx_host_name ON host(name);

COMMENT ON COLUMN host.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN host.environment_id IS 'environment.id';
COMMENT ON COLUMN host.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN host.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN host.checkcommand IS 'checkcommand.name';
COMMENT ON COLUMN host.checkcommand_id IS 'checkcommand.id';
COMMENT ON COLUMN host.check_timeperiod IS 'timeperiod.name';
COMMENT ON COLUMN host.check_timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN host.eventcommand IS 'eventcommand.name';
COMMENT ON COLUMN host.eventcommand_id IS 'eventcommand.id';
COMMENT ON COLUMN host.action_url_id IS 'action_url.id';
COMMENT ON COLUMN host.notes_url_id IS 'notes_url.id';
COMMENT ON COLUMN host.icon_image_id IS 'icon_image.id';
COMMENT ON COLUMN host.zone IS 'zone.name';
COMMENT ON COLUMN host.zone_id IS 'zone.id';
COMMENT ON COLUMN host.command_endpoint IS 'endpoint.name';
COMMENT ON COLUMN host.command_endpoint_id IS 'endpoint.id';

COMMENT ON INDEX idx_action_url_checksum IS 'cleanup';
COMMENT ON INDEX idx_notes_url_checksum IS 'cleanup';
COMMENT ON INDEX idx_icon_image_checksum IS 'cleanup';
COMMENT ON INDEX idx_host_display_name IS 'Host list filtered/ordered by display_name';
COMMENT ON INDEX idx_host_name_ci IS 'Host list filtered using quick search';
COMMENT ON INDEX idx_host_name IS 'Host list filtered/ordered by name; Host detail filter';

CREATE TABLE hostgroup (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_hostgroup PRIMARY KEY (id)
);

ALTER TABLE hostgroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_hostgroup_name ON hostgroup(name);

COMMENT ON COLUMN hostgroup.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN hostgroup.environment_id IS 'environment.id';
COMMENT ON COLUMN hostgroup.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN hostgroup.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN hostgroup.zone_id IS 'zone.id';

COMMENT ON INDEX idx_hostgroup_name IS 'Host/service/host group list filtered by host group name';

CREATE TABLE hostgroup_member (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  host_id bytea20 NOT NULL,
  hostgroup_id bytea20 NOT NULL,

  CONSTRAINT pk_hostgroup_member PRIMARY KEY (id)
);

ALTER TABLE hostgroup_member ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE hostgroup_member ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_member ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_member ALTER COLUMN hostgroup_id SET STORAGE PLAIN;

CREATE INDEX idx_hostgroup_member_host_id ON hostgroup_member(host_id, hostgroup_id);
CREATE INDEX idx_hostgroup_member_hostgroup_id ON hostgroup_member(hostgroup_id, host_id);

COMMENT ON COLUMN hostgroup_member.id IS 'sha1(environment.id + host_id + hostgroup_id)';
COMMENT ON COLUMN hostgroup_member.environment_id IS 'environment.id';
COMMENT ON COLUMN hostgroup_member.host_id IS 'host.id';
COMMENT ON COLUMN hostgroup_member.hostgroup_id IS 'hostgroup.id';

CREATE TABLE host_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  host_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_host_customvar PRIMARY KEY (id)
);

ALTER TABLE host_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE host_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE host_customvar ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE host_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_host_customvar_host_id ON host_customvar(host_id, customvar_id);
CREATE INDEX idx_host_customvar_customvar_id ON host_customvar(customvar_id, host_id);

COMMENT ON COLUMN host_customvar.id IS 'sha1(environment.id + host_id + customvar_id)';
COMMENT ON COLUMN host_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN host_customvar.host_id IS 'host.id';
COMMENT ON COLUMN host_customvar.customvar_id IS 'customvar.id';

CREATE TABLE hostgroup_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  hostgroup_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_hostgroup_customvar PRIMARY KEY (id)
);

ALTER TABLE hostgroup_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE hostgroup_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_customvar ALTER COLUMN hostgroup_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_hostgroup_customvar_hostgroup_id ON hostgroup_customvar(hostgroup_id, customvar_id);
CREATE INDEX idx_hostgroup_customvar_customvar_id ON hostgroup_customvar(customvar_id, hostgroup_id);

COMMENT ON COLUMN hostgroup_customvar.id IS 'sha1(environment.id + hostgroup_id + customvar_id)';
COMMENT ON COLUMN hostgroup_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN hostgroup_customvar.hostgroup_id IS 'hostgroup.id';
COMMENT ON COLUMN hostgroup_customvar.customvar_id IS 'customvar.id';

CREATE TABLE host_state (
  id bytea20 NOT NULL,
  host_id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  state_type state_type NOT NULL DEFAULT 'hard',
  soft_state tinyuint NOT NULL,
  hard_state tinyuint NOT NULL,
  previous_soft_state tinyuint NOT NULL,
  previous_hard_state tinyuint NOT NULL,
  attempt tinyuint NOT NULL,
  severity smalluint NOT NULL,

  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  performance_data text DEFAULT NULL,
  normalized_performance_data text DEFAULT NULL,

  check_commandline text DEFAULT NULL,

  is_problem boolenum NOT NULL DEFAULT 'n',
  is_handled boolenum NOT NULL DEFAULT 'n',
  is_reachable boolenum NOT NULL DEFAULT 'n',
  is_flapping boolenum NOT NULL DEFAULT 'n',
  is_overdue boolenum NOT NULL DEFAULT 'n',

  is_acknowledged acked NOT NULL DEFAULT 'n',
  acknowledgement_comment_id bytea20 DEFAULT NULL,
  last_comment_id bytea20 DEFAULT NULL,

  in_downtime boolenum NOT NULL DEFAULT 'n',

  execution_time uint DEFAULT NULL,
  latency uint DEFAULT NULL,
  timeout uint DEFAULT NULL,
  check_source text DEFAULT NULL,
  scheduling_source text DEFAULT NULL,

  last_update biguint DEFAULT NULL,
  last_state_change biguint NOT NULL,
  next_check biguint NOT NULL,
  next_update biguint NOT NULL,

  CONSTRAINT pk_host_state PRIMARY KEY (id)
);

ALTER TABLE host_state ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN acknowledgement_comment_id SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN last_comment_id SET STORAGE PLAIN;

CREATE UNIQUE INDEX idx_host_state_host_id ON host_state(host_id);
CREATE INDEX idx_host_state_is_problem ON host_state(is_problem, severity);
CREATE INDEX idx_host_state_severity ON host_state(severity);
CREATE INDEX idx_host_state_soft_state ON host_state(soft_state, last_state_change);
CREATE INDEX idx_host_state_last_state_change ON host_state(last_state_change);

COMMENT ON COLUMN host_state.id IS 'host.id';
COMMENT ON COLUMN host_state.host_id IS 'host.id';
COMMENT ON COLUMN host_state.environment_id IS 'environment.id';
COMMENT ON COLUMN host_state.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN host_state.acknowledgement_comment_id IS 'comment.id';
COMMENT ON COLUMN host_state.last_comment_id IS 'comment.id';

COMMENT ON INDEX idx_host_state_is_problem IS 'Host list filtered by is_problem ordered by severity';
COMMENT ON INDEX idx_host_state_severity IS 'Host list filtered/ordered by severity';
COMMENT ON INDEX idx_host_state_soft_state IS 'Host list filtered/ordered by soft_state; recently recovered filter';
COMMENT ON INDEX idx_host_state_last_state_change IS 'Host list filtered/ordered by last_state_change';

CREATE TABLE service (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,
  host_id bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,

  checkcommand citext NOT NULL,
  checkcommand_id bytea20 NOT NULL,

  max_check_attempts uint NOT NULL,

  check_timeperiod citext NOT NULL,
  check_timeperiod_id bytea20 DEFAULT NULL,

  check_timeout uint DEFAULT NULL,
  check_interval uint NOT NULL,
  check_retry_interval uint NOT NULL,

  active_checks_enabled boolenum NOT NULL DEFAULT 'n',
  passive_checks_enabled boolenum NOT NULL DEFAULT 'n',
  event_handler_enabled boolenum NOT NULL DEFAULT 'n',
  notifications_enabled boolenum NOT NULL DEFAULT 'n',

  flapping_enabled boolenum NOT NULL DEFAULT 'n',
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  perfdata_enabled boolenum NOT NULL DEFAULT 'n',

  eventcommand citext NOT NULL,
  eventcommand_id bytea20 DEFAULT NULL,

  is_volatile boolenum NOT NULL DEFAULT 'n',

  action_url_id bytea20 DEFAULT NULL,
  notes_url_id bytea20 DEFAULT NULL,
  notes text NOT NULL,
  icon_image_id bytea20 DEFAULT NULL,
  icon_image_alt varchar(32) NOT NULL,

  zone citext NOT NULL,
  zone_id bytea20 DEFAULT NULL,

  command_endpoint citext NOT NULL,
  command_endpoint_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_service PRIMARY KEY (id)
);

ALTER TABLE service ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN checkcommand_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN check_timeperiod_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN eventcommand_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN action_url_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN notes_url_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN icon_image_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE service ALTER COLUMN command_endpoint_id SET STORAGE PLAIN;

CREATE INDEX idx_service_display_name ON service(display_name);
CREATE INDEX idx_service_host_id ON service(host_id, display_name);
CREATE INDEX idx_service_name_ci ON service(name_ci);
CREATE INDEX idx_service_name ON service(name);

COMMENT ON COLUMN service.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN service.environment_id IS 'environment.id';
COMMENT ON COLUMN service.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN service.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN service.host_id IS 'sha1(host.id)';
COMMENT ON COLUMN service.checkcommand IS 'checkcommand.name';
COMMENT ON COLUMN service.checkcommand_id IS 'checkcommand.id';
COMMENT ON COLUMN service.check_timeperiod IS 'timeperiod.name';
COMMENT ON COLUMN service.check_timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN service.eventcommand IS 'eventcommand.name';
COMMENT ON COLUMN service.eventcommand_id IS 'eventcommand.id';
COMMENT ON COLUMN service.action_url_id IS 'action_url.id';
COMMENT ON COLUMN service.notes_url_id IS 'notes_url.id';
COMMENT ON COLUMN service.icon_image_id IS 'icon_image.id';
COMMENT ON COLUMN service.zone IS 'zone.name';
COMMENT ON COLUMN service.zone_id IS 'zone.id';
COMMENT ON COLUMN service.command_endpoint IS 'endpoint.name';
COMMENT ON COLUMN service.command_endpoint_id IS 'endpoint.id';

COMMENT ON INDEX idx_service_display_name IS 'Service list filtered/ordered by display_name';
COMMENT ON INDEX idx_service_host_id IS 'Service list filtered by host and ordered by display_name';
COMMENT ON INDEX idx_service_name_ci IS 'Service list filtered using quick search';
COMMENT ON INDEX idx_service_name IS 'Service list filtered/ordered by name; Service detail filter';

CREATE TABLE servicegroup (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_servicegroup PRIMARY KEY (id)
);

ALTER TABLE servicegroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN servicegroup.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN servicegroup.environment_id IS 'environment.id';
COMMENT ON COLUMN servicegroup.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN servicegroup.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN servicegroup.zone_id IS 'zone.id';

CREATE INDEX idx_servicegroup_name ON servicegroup(name);
COMMENT ON INDEX idx_servicegroup_name IS 'Host/service/service group list filtered by service group name';

CREATE TABLE servicegroup_member (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  service_id bytea20 NOT NULL,
  servicegroup_id bytea20 NOT NULL,

  CONSTRAINT pk_servicegroup_member PRIMARY KEY (id)
);

ALTER TABLE servicegroup_member ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE servicegroup_member ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_member ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_member ALTER COLUMN servicegroup_id SET STORAGE PLAIN;

CREATE INDEX idx_servicegroup_member_service_id ON servicegroup_member(service_id, servicegroup_id);
CREATE INDEX idx_servicegroup_member_servicegroup_id ON servicegroup_member(servicegroup_id, service_id);

COMMENT ON COLUMN servicegroup_member.id IS 'sha1(environment.id + servicegroup_id + service_id)';
COMMENT ON COLUMN servicegroup_member.environment_id IS 'environment.id';
COMMENT ON COLUMN servicegroup_member.service_id IS 'service.id';
COMMENT ON COLUMN servicegroup_member.servicegroup_id IS 'servicegroup.id';

CREATE TABLE service_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  service_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_service_customvar PRIMARY KEY (id)
);

ALTER TABLE service_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE service_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE service_customvar ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE service_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_service_customvar_service_id ON service_customvar(service_id, customvar_id);
CREATE INDEX idx_service_customvar_customvar_id ON service_customvar(customvar_id, service_id);

COMMENT ON COLUMN service_customvar.id IS 'sha1(environment.id + service_id + customvar_id)';
COMMENT ON COLUMN service_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN service_customvar.service_id IS 'service.id';
COMMENT ON COLUMN service_customvar.customvar_id IS 'customvar.id';

CREATE TABLE servicegroup_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  servicegroup_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_servicegroup_customvar PRIMARY KEY (id)
);

ALTER TABLE servicegroup_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE servicegroup_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_customvar ALTER COLUMN servicegroup_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_servicegroup_customvar_servicegroup_id ON servicegroup_customvar(servicegroup_id, customvar_id);
CREATE INDEX idx_servicegroup_customvar_customvar_id ON servicegroup_customvar(customvar_id, servicegroup_id);

COMMENT ON COLUMN servicegroup_customvar.id IS 'sha1(environment.id + servicegroup_id + customvar_id)';
COMMENT ON COLUMN servicegroup_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN servicegroup_customvar.servicegroup_id IS 'servicegroup.id';
COMMENT ON COLUMN servicegroup_customvar.customvar_id IS 'customvar.id';

CREATE TABLE service_state (
  id bytea20 NOT NULL,
  host_id bytea20 NOT NULL,
  service_id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  state_type state_type NOT NULL DEFAULT 'hard',
  soft_state tinyuint NOT NULL,
  hard_state tinyuint NOT NULL,
  previous_soft_state tinyuint NOT NULL,
  previous_hard_state tinyuint NOT NULL,
  attempt tinyuint NOT NULL,
  severity smalluint NOT NULL,

  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  performance_data text DEFAULT NULL,
  normalized_performance_data text DEFAULT NULL,

  check_commandline text DEFAULT NULL,

  is_problem boolenum NOT NULL DEFAULT 'n',
  is_handled boolenum NOT NULL DEFAULT 'n',
  is_reachable boolenum NOT NULL DEFAULT 'n',
  is_flapping boolenum NOT NULL DEFAULT 'n',
  is_overdue boolenum NOT NULL DEFAULT 'n',

  is_acknowledged acked NOT NULL DEFAULT 'n',
  acknowledgement_comment_id bytea20 DEFAULT NULL,
  last_comment_id bytea20 DEFAULT NULL,

  in_downtime boolenum NOT NULL DEFAULT 'n',

  execution_time uint DEFAULT NULL,
  latency uint DEFAULT NULL,
  timeout uint DEFAULT NULL,
  check_source text DEFAULT NULL,
  scheduling_source text DEFAULT NULL,

  last_update biguint DEFAULT NULL,
  last_state_change biguint NOT NULL,
  next_check biguint NOT NULL,
  next_update biguint NOT NULL,

  CONSTRAINT pk_service_state PRIMARY KEY (id)
);

ALTER TABLE service_state ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN acknowledgement_comment_id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN last_comment_id SET STORAGE PLAIN;

CREATE UNIQUE INDEX idx_service_state_service_id ON service_state(service_id);
CREATE INDEX idx_service_state_is_problem ON service_state(is_problem, severity);
CREATE INDEX idx_service_state_severity ON service_state(severity);
CREATE INDEX idx_service_state_soft_state ON service_state(soft_state, last_state_change);
CREATE INDEX idx_service_state_last_state_change ON service_state(last_state_change);

COMMENT ON COLUMN service_state.id IS 'service.id';
COMMENT ON COLUMN service_state.host_id IS 'host.id';
COMMENT ON COLUMN service_state.service_id IS 'service.id';
COMMENT ON COLUMN service_state.environment_id IS 'environment.id';
COMMENT ON COLUMN service_state.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN service_state.acknowledgement_comment_id IS 'comment.id';
COMMENT ON COLUMN service_state.last_comment_id IS 'comment.id';

COMMENT ON INDEX idx_service_state_is_problem IS 'Service list filtered by is_problem ordered by severity';
COMMENT ON INDEX idx_service_state_severity IS 'Service list filtered/ordered by severity';
COMMENT ON INDEX idx_service_state_soft_state IS 'Service list filtered/ordered by soft_state; recently recovered filter';
COMMENT ON INDEX idx_service_state_last_state_change IS 'Service list filtered/ordered by last_state_change';

CREATE TABLE endpoint (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,

  zone_id bytea20 NOT NULL,

  CONSTRAINT pk_endpoint PRIMARY KEY (id)
);

ALTER TABLE endpoint ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN endpoint.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN endpoint.environment_id IS 'environment.id';
COMMENT ON COLUMN endpoint.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN endpoint.zone_id IS 'zone.id';

CREATE TABLE environment (
  id bytea20 NOT NULL,
  name varchar(255) NOT NULL,

  CONSTRAINT pk_environment PRIMARY KEY (id)
);

ALTER TABLE environment ALTER COLUMN id SET STORAGE PLAIN;

COMMENT ON COLUMN environment.id IS 'sha1(Icinga CA public key)';

CREATE TABLE icingadb_instance (
  id bytea16 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  heartbeat biguint NOT NULL,
  responsible boolenum NOT NULL DEFAULT 'n',

  icinga2_version varchar(255) NOT NULL,
  icinga2_start_time biguint NOT NULL,
  icinga2_notifications_enabled boolenum NOT NULL DEFAULT 'n',
  icinga2_active_service_checks_enabled boolenum NOT NULL DEFAULT 'n',
  icinga2_active_host_checks_enabled boolenum NOT NULL DEFAULT 'n',
  icinga2_event_handlers_enabled boolenum NOT NULL DEFAULT 'n',
  icinga2_flap_detection_enabled boolenum NOT NULL DEFAULT 'n',
  icinga2_performance_data_enabled boolenum NOT NULL DEFAULT 'n',

  CONSTRAINT pk_icingadb_instance PRIMARY KEY (id)
);

ALTER TABLE icingadb_instance ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE icingadb_instance ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE icingadb_instance ALTER COLUMN endpoint_id SET STORAGE PLAIN;

COMMENT ON COLUMN icingadb_instance.id IS 'UUIDv4';
COMMENT ON COLUMN icingadb_instance.environment_id IS 'environment.id';
COMMENT ON COLUMN icingadb_instance.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN icingadb_instance.heartbeat IS '*nix timestamp';

CREATE TABLE checkcommand (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  zone_id bytea20 DEFAULT NULL,

  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  command text NOT NULL,
  timeout uint NOT NULL,

  CONSTRAINT pk_checkcommand PRIMARY KEY (id)
);

ALTER TABLE checkcommand ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand.id IS 'sha1(environment.id + type + name)';
COMMENT ON COLUMN checkcommand.environment_id IS 'env.id';
COMMENT ON COLUMN checkcommand.zone_id IS 'zone.id';
COMMENT ON COLUMN checkcommand.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN checkcommand.properties_checksum IS 'sha1(all properties)';

CREATE TABLE checkcommand_argument (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  checkcommand_id bytea20 NOT NULL,
  argument_key varchar(64) NOT NULL,

  properties_checksum bytea20 NOT NULL,

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override citext DEFAULT NULL,
  repeat_key boolenum NOT NULL DEFAULT 'n',
  required boolenum NOT NULL DEFAULT 'n',
  set_if varchar(255) DEFAULT NULL,
  skip_key boolenum NOT NULL DEFAULT 'n',

  CONSTRAINT pk_checkcommand_argument PRIMARY KEY (id)
);

ALTER TABLE checkcommand_argument ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN checkcommand_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN argument_key SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand_argument.id IS 'sha1(environment.id + checkcommand_id + argument_key)';
COMMENT ON COLUMN checkcommand_argument.environment_id IS 'env.id';
COMMENT ON COLUMN checkcommand_argument.checkcommand_id IS 'checkcommand.id';
COMMENT ON COLUMN checkcommand_argument.properties_checksum IS 'sha1(all properties)';

CREATE TABLE checkcommand_envvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  checkcommand_id bytea20 NOT NULL,
  envvar_key varchar(64) NOT NULL,

  properties_checksum bytea20 NOT NULL,

  envvar_value text NOT NULL,

  CONSTRAINT pk_checkcommand_envvar PRIMARY KEY (id)
);

ALTER TABLE checkcommand_envvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand_envvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_envvar ALTER COLUMN checkcommand_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_envvar ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand_envvar.id IS 'sha1(environment.id + checkcommand_id + envvar_key)';
COMMENT ON COLUMN checkcommand_envvar.environment_id IS 'env.id';
COMMENT ON COLUMN checkcommand_envvar.checkcommand_id IS 'checkcommand.id';
COMMENT ON COLUMN checkcommand_envvar.properties_checksum IS 'sha1(all properties)';

CREATE TABLE checkcommand_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,

  checkcommand_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_checkcommand_customvar PRIMARY KEY (id)
);

ALTER TABLE checkcommand_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_customvar ALTER COLUMN checkcommand_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_checkcommand_customvar_checkcommand_id ON checkcommand_customvar(checkcommand_id, customvar_id);
CREATE INDEX idx_checkcommand_customvar_customvar_id ON checkcommand_customvar(customvar_id, checkcommand_id);

COMMENT ON COLUMN checkcommand_customvar.id IS 'sha1(environment.id + checkcommand_id + customvar_id)';
COMMENT ON COLUMN checkcommand_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN checkcommand_customvar.checkcommand_id IS 'checkcommand.id';
COMMENT ON COLUMN checkcommand_customvar.customvar_id IS 'customvar.id';

CREATE TABLE eventcommand (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  zone_id bytea20 DEFAULT NULL,

  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  command text NOT NULL,
  timeout smalluint NOT NULL,

  CONSTRAINT pk_eventcommand PRIMARY KEY (id)
);

ALTER TABLE eventcommand ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand.id IS 'sha1(environment.id + type + name)';
COMMENT ON COLUMN eventcommand.environment_id IS 'env.id';
COMMENT ON COLUMN eventcommand.zone_id IS 'zone.id';
COMMENT ON COLUMN eventcommand.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN eventcommand.properties_checksum IS 'sha1(all properties)';

CREATE TABLE eventcommand_argument (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  eventcommand_id bytea20 NOT NULL,
  argument_key varchar(64) NOT NULL,

  properties_checksum bytea20 NOT NULL,

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override citext DEFAULT NULL,
  repeat_key boolenum NOT NULL DEFAULT 'n',
  required boolenum NOT NULL DEFAULT 'n',
  set_if varchar(255) DEFAULT NULL,
  skip_key boolenum NOT NULL DEFAULT 'n',

  CONSTRAINT pk_eventcommand_argument PRIMARY KEY (id)
);

ALTER TABLE eventcommand_argument ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand_argument ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_argument ALTER COLUMN eventcommand_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_argument ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand_argument.id IS 'sha1(environment.id + eventcommand_id + argument_key)';
COMMENT ON COLUMN eventcommand_argument.environment_id IS 'env.id';
COMMENT ON COLUMN eventcommand_argument.eventcommand_id IS 'eventcommand.id';
COMMENT ON COLUMN eventcommand_argument.properties_checksum IS 'sha1(all properties)';

CREATE TABLE eventcommand_envvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  eventcommand_id bytea20 NOT NULL,
  envvar_key varchar(64) NOT NULL,

  properties_checksum bytea20 NOT NULL,

  envvar_value text NOT NULL,

  CONSTRAINT pk_eventcommand_envvar PRIMARY KEY (id)
);

ALTER TABLE eventcommand_envvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand_envvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_envvar ALTER COLUMN eventcommand_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_envvar ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand_envvar.id IS 'sha1(environment.id + eventcommand_id + envvar_key)';
COMMENT ON COLUMN eventcommand_envvar.environment_id IS 'env.id';
COMMENT ON COLUMN eventcommand_envvar.eventcommand_id IS 'eventcommand.id';
COMMENT ON COLUMN eventcommand_envvar.properties_checksum IS 'sha1(all properties)';

CREATE TABLE eventcommand_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  eventcommand_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_eventcommand_customvar PRIMARY KEY (id)
);

ALTER TABLE eventcommand_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_customvar ALTER COLUMN eventcommand_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_eventcommand_customvar_eventcommand_id ON eventcommand_customvar(eventcommand_id, customvar_id);
CREATE INDEX idx_eventcommand_customvar_customvar_id ON eventcommand_customvar(customvar_id, eventcommand_id);

COMMENT ON COLUMN eventcommand_customvar.id IS 'sha1(environment.id + eventcommand_id + customvar_id)';
COMMENT ON COLUMN eventcommand_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN eventcommand_customvar.eventcommand_id IS 'eventcommand.id';
COMMENT ON COLUMN eventcommand_customvar.customvar_id IS 'customvar.id';

CREATE TABLE notificationcommand (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  zone_id bytea20 DEFAULT NULL,

  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  command text NOT NULL,
  timeout smalluint NOT NULL,

  CONSTRAINT pk_notificationcommand PRIMARY KEY (id)
);

ALTER TABLE notificationcommand ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand.id IS 'sha1(environment.id + type + name)';
COMMENT ON COLUMN notificationcommand.environment_id IS 'env.id';
COMMENT ON COLUMN notificationcommand.zone_id IS 'zone.id';
COMMENT ON COLUMN notificationcommand.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN notificationcommand.properties_checksum IS 'sha1(all properties)';

CREATE TABLE notificationcommand_argument (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notificationcommand_id bytea20 NOT NULL,
  argument_key varchar(64) NOT NULL,

  properties_checksum bytea20 NOT NULL,

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override citext DEFAULT NULL,
  repeat_key boolenum NOT NULL DEFAULT 'n',
  required boolenum NOT NULL DEFAULT 'n',
  set_if varchar(255) DEFAULT NULL,
  skip_key boolenum NOT NULL DEFAULT 'n',

  CONSTRAINT pk_notificationcommand_argument PRIMARY KEY (id)
);

ALTER TABLE notificationcommand_argument ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_argument ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_argument ALTER COLUMN notificationcommand_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_argument ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand_argument.id IS 'sha1(environment.id + notificationcommand_id + argument_key)';
COMMENT ON COLUMN notificationcommand_argument.environment_id IS 'env.id';
COMMENT ON COLUMN notificationcommand_argument.notificationcommand_id IS 'notificationcommand.id';
COMMENT ON COLUMN notificationcommand_argument.properties_checksum IS 'sha1(all properties)';

CREATE TABLE notificationcommand_envvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notificationcommand_id bytea20 NOT NULL,
  envvar_key varchar(64) NOT NULL,

  properties_checksum bytea20 NOT NULL,

  envvar_value text NOT NULL,

  CONSTRAINT pk_notificationcommand_envvar PRIMARY KEY (id)
);

ALTER TABLE notificationcommand_envvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_envvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_envvar ALTER COLUMN notificationcommand_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_envvar ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand_envvar.id IS 'sha1(environment.id + notificationcommand_id + envvar_key)';
COMMENT ON COLUMN notificationcommand_envvar.environment_id IS 'env.id';
COMMENT ON COLUMN notificationcommand_envvar.notificationcommand_id IS 'notificationcommand.id';
COMMENT ON COLUMN notificationcommand_envvar.properties_checksum IS 'sha1(all properties)';

CREATE TABLE notificationcommand_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notificationcommand_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_notificationcommand_customvar PRIMARY KEY (id)
);

ALTER TABLE notificationcommand_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_customvar ALTER COLUMN notificationcommand_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_notificationcommand_customvar_notificationcommand_id ON notificationcommand_customvar(notificationcommand_id, customvar_id);
CREATE INDEX idx_notificationcommand_customvar_customvar_id ON notificationcommand_customvar(customvar_id, notificationcommand_id);

COMMENT ON COLUMN notificationcommand_customvar.id IS 'sha1(environment.id + notificationcommand_id + customvar_id)';
COMMENT ON COLUMN notificationcommand_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN notificationcommand_customvar.notificationcommand_id IS 'notificationcommand.id';
COMMENT ON COLUMN notificationcommand_customvar.customvar_id IS 'customvar.id';

CREATE TABLE comment (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,

  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,
  name varchar(548) NOT NULL,

  author citext NOT NULL,
  text text NOT NULL,
  entry_type comment_type NOT NULL DEFAULT 'comment',
  entry_time biguint NOT NULL,
  is_persistent boolenum NOT NULL DEFAULT 'n',
  is_sticky boolenum NOT NULL DEFAULT 'n',
  expire_time biguint DEFAULT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_comment PRIMARY KEY (id)
);

ALTER TABLE comment ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE comment ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE comment ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE comment ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE comment ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE comment ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE comment ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_comment_name ON comment(name);
CREATE INDEX idx_comment_entry_time ON comment(entry_time);
CREATE INDEX idx_comment_author ON comment(author);
CREATE INDEX idx_comment_expire_time ON comment(expire_time);

COMMENT ON COLUMN comment.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN comment.environment_id IS 'environment.id';
COMMENT ON COLUMN comment.host_id IS 'host.id';
COMMENT ON COLUMN comment.service_id IS 'service.id';
COMMENT ON COLUMN comment.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN comment.name IS '255+1+255+1+36, i.e. "host.name!service.name!UUID"';
COMMENT ON COLUMN comment.zone_id IS 'zone.id';

COMMENT ON INDEX idx_comment_name IS 'Comment detail filter';
COMMENT ON INDEX idx_comment_entry_time IS 'Comment list fileted/ordered by entry_time';
COMMENT ON INDEX idx_comment_author IS 'Comment list filtered/ordered by author';
COMMENT ON INDEX idx_comment_expire_time IS 'Comment list filtered/ordered by expire_time';

CREATE TABLE downtime (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,

  triggered_by_id bytea20 DEFAULT NULL,
  parent_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,
  name varchar(548) NOT NULL,

  author citext NOT NULL,
  comment text NOT NULL,
  entry_time biguint NOT NULL,
  scheduled_start_time biguint NOT NULL,
  scheduled_end_time biguint NOT NULL,
  scheduled_duration biguint NOT NULL,
  is_flexible boolenum NOT NULL DEFAULT 'n',
  flexible_duration biguint NOT NULL,

  is_in_effect boolenum NOT NULL DEFAULT 'n',
  start_time biguint DEFAULT NULL,
  end_time biguint DEFAULT NULL,
  duration biguint NOT NULL,
  scheduled_by varchar(767) DEFAULT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_downtime PRIMARY KEY (id)
);

ALTER TABLE downtime ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN triggered_by_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN parent_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_downtime_is_in_effect ON downtime(is_in_effect, start_time);
CREATE INDEX idx_downtime_name ON downtime(name);
CREATE INDEX idx_downtime_entry_time ON downtime(entry_time);
CREATE INDEX idx_downtime_start_time ON downtime(start_time);
CREATE INDEX idx_downtime_end_time ON downtime(end_time);
CREATE INDEX idx_downtime_scheduled_start_time ON downtime(scheduled_start_time);
CREATE INDEX idx_downtime_scheduled_end_time ON downtime(scheduled_end_time);
CREATE INDEX idx_downtime_author ON downtime(author);
CREATE INDEX idx_downtime_duration ON downtime(duration);

COMMENT ON COLUMN downtime.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN downtime.environment_id IS 'environment.id';
COMMENT ON COLUMN downtime.triggered_by_id IS 'The ID of the downtime that triggered this downtime. This is set when creating downtimes on a host or service higher up in the dependency chain using the "child_option" "DowntimeTriggeredChildren" and can also be set manually via the API.';
COMMENT ON COLUMN downtime.parent_id IS 'For service downtimes, the ID of the host downtime that created this downtime by using the "all_services" flag of the schedule-downtime API.';
COMMENT ON COLUMN downtime.host_id IS 'host.id';
COMMENT ON COLUMN downtime.service_id IS 'service.id';
COMMENT ON COLUMN downtime.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN downtime.name IS '255+1+255+1+36, i.e. "host.name!service.name!UUID"';
COMMENT ON COLUMN downtime.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN downtime.start_time IS 'Time when the host went into a problem state during the downtimes timeframe';
COMMENT ON COLUMN downtime.end_time IS 'Problem state assumed: scheduled_end_time if fixed, start_time + flexible_duration otherwise';
COMMENT ON COLUMN downtime.duration IS 'Duration of the downtime: When the downtime is flexible, this is the same as flexible_duration otherwise scheduled_duration';
COMMENT ON COLUMN downtime.scheduled_by IS 'Name of the ScheduledDowntime which created this Downtime. 255+1+255+1+255, i.e. "host.name!service.name!scheduled-downtime-name"';
COMMENT ON COLUMN downtime.zone_id IS 'zone.id';

COMMENT ON INDEX idx_downtime_is_in_effect IS 'Downtime list filtered/ordered by severity';
COMMENT ON INDEX idx_downtime_name IS 'Downtime detail filter';
COMMENT ON INDEX idx_downtime_entry_time IS 'Downtime list filtered/ordered by entry_time';
COMMENT ON INDEX idx_downtime_start_time IS 'Downtime list filtered/ordered by start_time';
COMMENT ON INDEX idx_downtime_end_time IS 'Downtime list filtered/ordered by end_time';
COMMENT ON INDEX idx_downtime_scheduled_start_time IS 'Downtime list filtered/ordered by scheduled_start_time';
COMMENT ON INDEX idx_downtime_scheduled_end_time IS 'Downtime list filtered/ordered by scheduled_end_time';
COMMENT ON INDEX idx_downtime_author IS 'Downtime list filtered/ordered by author';
COMMENT ON INDEX idx_downtime_duration IS 'Downtime list filtered/ordered by duration';

CREATE TABLE notification (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,

  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,
  notificationcommand_id bytea20 NOT NULL,

  times_begin uint DEFAULT NULL,
  times_end uint DEFAULT NULL,
  notification_interval uint NOT NULL,
  timeperiod_id bytea20 DEFAULT NULL,

  states tinyuint NOT NULL,
  types smalluint NOT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_notification PRIMARY KEY (id)
);

ALTER TABLE notification ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN notificationcommand_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_host_id ON notification(host_id);
CREATE INDEX idx_notification_service_id ON notification(service_id);

COMMENT ON COLUMN notification.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN notification.environment_id IS 'environment.id';
COMMENT ON COLUMN notification.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN notification.host_id IS 'host.id';
COMMENT ON COLUMN notification.service_id IS 'service.id';
COMMENT ON COLUMN notification.notificationcommand_id IS 'command.id';
COMMENT ON COLUMN notification.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN notification.zone_id IS 'zone.id';

CREATE TABLE notification_user (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notification_id bytea20 NOT NULL,
  user_id bytea20 NOT NULL,

  CONSTRAINT pk_notification_user PRIMARY KEY (id)
);

ALTER TABLE notification_user ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_user ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_user ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_user ALTER COLUMN user_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_user_user_id ON notification_user(user_id, notification_id);
CREATE INDEX idx_notification_user_notification_id ON notification_user(notification_id, user_id);

COMMENT ON COLUMN notification_user.id IS 'sha1(environment.id + notification_id + user_id)';
COMMENT ON COLUMN notification_user.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_user.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_user.user_id IS 'user.id';

CREATE TABLE notification_usergroup (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notification_id bytea20 NOT NULL,
  usergroup_id bytea20 NOT NULL,

  CONSTRAINT pk_notification_usergroup PRIMARY KEY (id)
);

ALTER TABLE notification_usergroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_usergroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_usergroup ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_usergroup ALTER COLUMN usergroup_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_usergroup_usergroup_id ON notification_usergroup(usergroup_id, notification_id);
CREATE INDEX idx_notification_usergroup_notification_id ON notification_usergroup(notification_id, usergroup_id);

COMMENT ON COLUMN notification_usergroup.id IS 'sha1(environment.id + notification_id + usergroup_id)';
COMMENT ON COLUMN notification_usergroup.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_usergroup.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_usergroup.usergroup_id IS 'usergroup.id';

CREATE TABLE notification_recipient (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notification_id bytea20 NOT NULL,
  user_id bytea20 NULL,
  usergroup_id bytea20 NULL,

  CONSTRAINT pk_notification_recipient PRIMARY KEY (id)
);

ALTER TABLE notification_recipient ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_recipient ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_recipient ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_recipient ALTER COLUMN user_id SET STORAGE PLAIN;
ALTER TABLE notification_recipient ALTER COLUMN usergroup_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_recipient_user_id ON notification_recipient(user_id, notification_id);
CREATE INDEX idx_notification_recipient_notification_id_user ON notification_recipient(notification_id, user_id);
CREATE INDEX idx_notification_recipient_usergroup_id ON notification_recipient(usergroup_id, notification_id);
CREATE INDEX idx_notification_recipient_notification_id_usergroup ON notification_recipient(notification_id, usergroup_id);

COMMENT ON COLUMN notification_recipient.id IS 'sha1(environment.id + notification_id + (user_id | usergroup_id))';
COMMENT ON COLUMN notification_recipient.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_recipient.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_recipient.user_id IS 'user.id';
COMMENT ON COLUMN notification_recipient.usergroup_id IS 'usergroup.id';

CREATE TABLE notification_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notification_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_notification_customvar PRIMARY KEY (id)
);

ALTER TABLE notification_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_customvar ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_customvar_notification_id ON notification_customvar(notification_id, customvar_id);
CREATE INDEX idx_notification_customvar_customvar_id ON notification_customvar(customvar_id, notification_id);

COMMENT ON COLUMN notification_customvar.id IS 'sha1(environment.id + notification_id + customvar_id)';
COMMENT ON COLUMN notification_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_customvar.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_customvar.customvar_id IS 'customvar.id';

CREATE TABLE icon_image (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  icon_image citext NOT NULL,

  CONSTRAINT pk_icon_image PRIMARY KEY (id)
);

ALTER TABLE icon_image ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE icon_image ALTER COLUMN environment_id SET STORAGE PLAIN;

CREATE INDEX idx_icon_image ON icon_image(icon_image);

COMMENT ON COLUMN icon_image.id IS 'sha1(environment.id + icon_image)';
COMMENT ON COLUMN icon_image.environment_id IS 'environment.id';

CREATE TABLE action_url (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  action_url citext NOT NULL,

  CONSTRAINT pk_action_url PRIMARY KEY (id)
);

ALTER TABLE action_url ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE action_url ALTER COLUMN environment_id SET STORAGE PLAIN;

CREATE INDEX idx_action_url ON action_url(action_url);

COMMENT ON COLUMN action_url.id IS 'sha1(environment.id + action_url)';
COMMENT ON COLUMN action_url.environment_id IS 'environment.id';

CREATE TABLE notes_url (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notes_url citext NOT NULL,

  CONSTRAINT pk_notes_url PRIMARY KEY (id)
);

ALTER TABLE notes_url ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notes_url ALTER COLUMN environment_id SET STORAGE PLAIN;

CREATE INDEX idx_notes_url ON notes_url(notes_url);

COMMENT ON COLUMN notes_url.id IS 'sha1(environment.id + notes_url)';
COMMENT ON COLUMN notes_url.environment_id IS 'environment.id';

CREATE TABLE timeperiod (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,

  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,
  prefer_includes boolenum NOT NULL DEFAULT 'n',

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_timeperiod PRIMARY KEY (id)
);

ALTER TABLE timeperiod ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN timeperiod.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN timeperiod.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN timeperiod.zone_id IS 'zone.id';

CREATE TABLE timeperiod_range (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  timeperiod_id bytea20 NOT NULL,
  range_key citext NOT NULL,

  range_value varchar(255) NOT NULL,

  CONSTRAINT pk_timeperiod_range PRIMARY KEY (id)
);

ALTER TABLE timeperiod_range ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_range ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_range ALTER COLUMN timeperiod_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_range.id IS 'sha1(environment.id + range_id + timeperiod_id)';
COMMENT ON COLUMN timeperiod_range.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod_range.timeperiod_id IS 'timeperiod.id';

CREATE TABLE timeperiod_override_include (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  timeperiod_id bytea20 NOT NULL,
  override_id bytea20 NOT NULL,

  CONSTRAINT pk_timeperiod_override_include PRIMARY KEY (id)
);

ALTER TABLE timeperiod_override_include ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_include ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_include ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_include ALTER COLUMN override_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_override_include.id IS 'sha1(environment.id + include_id + timeperiod_id)';
COMMENT ON COLUMN timeperiod_override_include.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod_override_include.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN timeperiod_override_include.override_id IS 'timeperiod.id';

CREATE TABLE timeperiod_override_exclude (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  timeperiod_id bytea20 NOT NULL,
  override_id bytea20 NOT NULL,

  CONSTRAINT pk_timeperiod_override_exclude PRIMARY KEY (id)
);

ALTER TABLE timeperiod_override_exclude ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_exclude ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_exclude ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_exclude ALTER COLUMN override_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_override_exclude.id IS 'sha1(environment.id + exclude_id + timeperiod_id)';
COMMENT ON COLUMN timeperiod_override_exclude.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod_override_exclude.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN timeperiod_override_exclude.override_id IS 'timeperiod.id';

CREATE TABLE timeperiod_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  timeperiod_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_timeperiod_customvar PRIMARY KEY (id)
);

ALTER TABLE timeperiod_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_customvar ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_timeperiod_customvar_timeperiod_id ON timeperiod_customvar(timeperiod_id, customvar_id);
CREATE INDEX idx_timeperiod_customvar_customvar_id ON timeperiod_customvar(customvar_id, timeperiod_id);

COMMENT ON COLUMN timeperiod_customvar.id IS 'sha1(environment.id + timeperiod_id + customvar_id)';
COMMENT ON COLUMN timeperiod_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN timeperiod_customvar.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN timeperiod_customvar.customvar_id IS 'customvar.id';

CREATE TABLE customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  value text NOT NULL,

  CONSTRAINT pk_customvar PRIMARY KEY (id)
);

ALTER TABLE customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE customvar ALTER COLUMN name_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN customvar.id IS 'sha1(environment.id + name + value)';
COMMENT ON COLUMN customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN customvar.name_checksum IS 'sha1(name)';

CREATE TABLE customvar_flat (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,
  flatname_checksum bytea20 NOT NULL,

  flatname varchar(512) NOT NULL,
  flatvalue text NOT NULL,

  CONSTRAINT pk_customvar_flat PRIMARY KEY (id)
);

ALTER TABLE customvar_flat ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE customvar_flat ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE customvar_flat ALTER COLUMN customvar_id SET STORAGE PLAIN;
ALTER TABLE customvar_flat ALTER COLUMN flatname_checksum SET STORAGE PLAIN;

CREATE INDEX idx_customvar_flat_customvar_id ON customvar_flat(customvar_id);

COMMENT ON COLUMN customvar_flat.id IS 'sha1(environment.id + flatname + flatvalue)';
COMMENT ON COLUMN customvar_flat.environment_id IS 'environment.id';
COMMENT ON COLUMN customvar_flat.customvar_id IS 'sha1(customvar.id)';
COMMENT ON COLUMN customvar_flat.flatname_checksum IS 'sha1(flatname after conversion)';
COMMENT ON COLUMN customvar_flat.flatname IS 'Path converted with `.` and `[ ]`';

CREATE TABLE "user" (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,

  email varchar(255) NOT NULL,
  pager varchar(255) NOT NULL,

  notifications_enabled boolenum NOT NULL DEFAULT 'n',

  timeperiod_id bytea20 DEFAULT NULL,

  states tinyuint NOT NULL,
  types smalluint NOT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_user PRIMARY KEY (id)
);

ALTER TABLE "user" ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_user_display_name ON "user"(display_name);
CREATE INDEX idx_user_name_ci ON "user"(name_ci);
CREATE INDEX idx_user_name ON "user"(name);

COMMENT ON COLUMN "user".id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN "user".environment_id IS 'environment.id';
COMMENT ON COLUMN "user".name_checksum IS 'sha1(name)';
COMMENT ON COLUMN "user".properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN "user".timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN "user".zone_id IS 'zone.id';

COMMENT ON INDEX idx_user_display_name IS 'User list filtered/ordered by display_name';
COMMENT ON INDEX idx_user_name_ci IS 'User list filtered using quick search';
COMMENT ON INDEX idx_user_name IS 'User list filtered/ordered by name; User detail filter';

CREATE TABLE usergroup (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,
  display_name citext NOT NULL,

  zone_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_usergroup PRIMARY KEY (id)
);

ALTER TABLE usergroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_usergroup_display_name ON usergroup(display_name);
CREATE INDEX idx_usergroup_name_ci ON usergroup(name_ci);
CREATE INDEX idx_usergroup_name ON usergroup(name);

COMMENT ON COLUMN usergroup.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN usergroup.environment_id IS 'environment.id';
COMMENT ON COLUMN usergroup.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN usergroup.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN usergroup.zone_id IS 'zone.id';

COMMENT ON INDEX idx_usergroup_display_name IS 'Usergroup list filtered/ordered by display_name';
COMMENT ON INDEX idx_usergroup_name_ci IS 'Usergroup list filtered using quick search';
COMMENT ON INDEX idx_usergroup_name IS 'Usergroup list filtered/ordered by name; User detail filter';

CREATE TABLE usergroup_member (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  user_id bytea20 NOT NULL,
  usergroup_id bytea20 NOT NULL,

  CONSTRAINT pk_usergroup_member PRIMARY KEY (id)
);

ALTER TABLE usergroup_member ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE usergroup_member ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE usergroup_member ALTER COLUMN user_id SET STORAGE PLAIN;
ALTER TABLE usergroup_member ALTER COLUMN usergroup_id SET STORAGE PLAIN;

CREATE INDEX idx_usergroup_member_user_id ON usergroup_member(user_id, usergroup_id);
CREATE INDEX idx_usergroup_member_usergroup_id ON usergroup_member(usergroup_id, user_id);

COMMENT ON COLUMN usergroup_member.id IS 'sha1(environment.id + usergroup_id + user_id)';
COMMENT ON COLUMN usergroup_member.environment_id IS 'environment.id';
COMMENT ON COLUMN usergroup_member.user_id IS 'user.id';
COMMENT ON COLUMN usergroup_member.usergroup_id IS 'usergroup.id';

CREATE TABLE user_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  user_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_user_customvar PRIMARY KEY (id)
);

ALTER TABLE user_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE user_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE user_customvar ALTER COLUMN user_id SET STORAGE PLAIN;
ALTER TABLE user_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_user_customvar_user_id ON user_customvar(user_id, customvar_id);
CREATE INDEX idx_user_customvar_customvar_id ON user_customvar(customvar_id, user_id);

COMMENT ON COLUMN user_customvar.id IS 'sha1(environment.id + user_id + customvar_id)';
COMMENT ON COLUMN user_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN user_customvar.user_id IS 'user.id';
COMMENT ON COLUMN user_customvar.customvar_id IS 'customvar.id';

CREATE TABLE usergroup_customvar (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  usergroup_id bytea20 NOT NULL,
  customvar_id bytea20 NOT NULL,

  CONSTRAINT pk_usergroup_customvar PRIMARY KEY (id)
);

ALTER TABLE usergroup_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE usergroup_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE usergroup_customvar ALTER COLUMN usergroup_id SET STORAGE PLAIN;
ALTER TABLE usergroup_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_usergroup_customvar_usergroup_id ON usergroup_customvar(usergroup_id, customvar_id);
CREATE INDEX idx_usergroup_customvar_customvar_id ON usergroup_customvar(customvar_id, usergroup_id);

COMMENT ON COLUMN usergroup_customvar.id IS 'sha1(environment.id + usergroup_id + customvar_id)';
COMMENT ON COLUMN usergroup_customvar.environment_id IS 'environment.id';
COMMENT ON COLUMN usergroup_customvar.usergroup_id IS 'usergroup.id';
COMMENT ON COLUMN usergroup_customvar.customvar_id IS 'customvar.id';

CREATE TABLE zone (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci citext NOT NULL,

  is_global boolenum NOT NULL DEFAULT 'n',
  parent_id bytea20 DEFAULT NULL,

  depth tinyuint NOT NULL,

  CONSTRAINT pk_zone PRIMARY KEY (id)
);

ALTER TABLE zone ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN parent_id SET STORAGE PLAIN;

CREATE UNIQUE INDEX idx_environment_id_id ON zone(environment_id, id);
CREATE INDEX idx_zone_parent_id ON zone(parent_id);

COMMENT ON COLUMN zone.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN zone.environment_id IS 'environment.id';
COMMENT ON COLUMN zone.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN zone.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN zone.parent_id IS 'zone.id';

CREATE TABLE notification_history (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,
  notification_id bytea20 NOT NULL,

  type notification_type NOT NULL DEFAULT 'downtime_start',
  send_time biguint NOT NULL,
  state tinyuint NOT NULL,
  previous_hard_state tinyuint NOT NULL,
  author text NOT NULL,
  "text" text NOT NULL,
  users_notified smalluint NOT NULL,

  CONSTRAINT pk_notification_history PRIMARY KEY (id)
);

ALTER TABLE notification_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN notification_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_history_send_time ON notification_history(send_time DESC);

COMMENT ON COLUMN notification_history.id IS 'sha1(environment.name + notification.name + type + send_time)';
COMMENT ON COLUMN notification_history.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN notification_history.host_id IS 'host.id';
COMMENT ON COLUMN notification_history.service_id IS 'service.id';
COMMENT ON COLUMN notification_history.notification_id IS 'notification.id';

COMMENT ON INDEX idx_notification_history_send_time IS 'Notification list filtered/ordered by send_time';

CREATE TABLE user_notification_history (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  notification_history_id bytea20 NOT NULL,
  user_id bytea20 NOT NULL,

  CONSTRAINT pk_user_notification_history PRIMARY KEY (id),

  CONSTRAINT fk_user_notification_history_notification_history FOREIGN KEY (notification_history_id) REFERENCES notification_history (id) ON DELETE CASCADE
);

ALTER TABLE user_notification_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE user_notification_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE user_notification_history ALTER COLUMN notification_history_id SET STORAGE PLAIN;
ALTER TABLE user_notification_history ALTER COLUMN user_id SET STORAGE PLAIN;

COMMENT ON COLUMN user_notification_history.id IS 'sha1(notification_history_id + user_id)';
COMMENT ON COLUMN user_notification_history.environment_id IS 'environment.id';
COMMENT ON COLUMN user_notification_history.notification_history_id IS 'UUID notification_history.id';
COMMENT ON COLUMN user_notification_history.user_id IS 'user.id';

CREATE TABLE state_history (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  event_time biguint NOT NULL,
  state_type state_type NOT NULL DEFAULT 'hard',
  soft_state tinyuint NOT NULL,
  hard_state tinyuint NOT NULL,
  previous_soft_state tinyuint NOT NULL,
  previous_hard_state tinyuint NOT NULL,
  attempt tinyuint NOT NULL,
  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  max_check_attempts uint NOT NULL,
  check_source text DEFAULT NULL,
  scheduling_source text DEFAULT NULL,

  CONSTRAINT pk_state_history PRIMARY KEY (id)
);

ALTER TABLE state_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN state_history.id IS 'sha1(environment.name + host|service.name + event_time)';
COMMENT ON COLUMN state_history.environment_id IS 'environment.id';
COMMENT ON COLUMN state_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN state_history.host_id IS 'host.id';
COMMENT ON COLUMN state_history.service_id IS 'service.id';

CREATE TABLE downtime_history (
  downtime_id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  triggered_by_id bytea20 DEFAULT NULL,
  parent_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  entry_time biguint NOT NULL,
  author citext NOT NULL,
  cancelled_by citext DEFAULT NULL,
  comment text NOT NULL,
  is_flexible boolenum NOT NULL DEFAULT 'n',
  flexible_duration biguint NOT NULL,
  scheduled_start_time biguint NOT NULL,
  scheduled_end_time biguint NOT NULL,
  start_time biguint NOT NULL,
  end_time biguint NOT NULL,
  scheduled_by varchar(767) DEFAULT NULL,
  has_been_cancelled boolenum NOT NULL DEFAULT 'n',
  trigger_time biguint NOT NULL,
  cancel_time biguint DEFAULT NULL,

  CONSTRAINT pk_downtime_history PRIMARY KEY (downtime_id)
);

ALTER TABLE downtime_history ALTER COLUMN downtime_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN triggered_by_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN parent_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN downtime_history.downtime_id IS 'downtime.id';
COMMENT ON COLUMN downtime_history.environment_id IS 'environment.id';
COMMENT ON COLUMN downtime_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN downtime_history.triggered_by_id IS 'The ID of the downtime that triggered this downtime. This is set when creating downtimes on a host or service higher up in the dependency chain using the "child_option" "DowntimeTriggeredChildren" and can also be set manually via the API.';
COMMENT ON COLUMN downtime_history.parent_id IS 'For service downtimes, the ID of the host downtime that created this downtime by using the "all_services" flag of the schedule-downtime API.';
COMMENT ON COLUMN downtime_history.host_id IS 'host.id';
COMMENT ON COLUMN downtime_history.service_id IS 'service.id';
COMMENT ON COLUMN downtime_history.start_time IS 'Time when the host went into a problem state during the downtimes timeframe';
COMMENT ON COLUMN downtime_history.end_time IS 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise';
COMMENT ON COLUMN downtime_history.scheduled_by IS 'Name of the ScheduledDowntime which created this Downtime. 255+1+255+1+255, i.e. "host.name!service.name!scheduled-downtime-name"';

CREATE TABLE comment_history (
  comment_id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  entry_time biguint NOT NULL,
  author citext NOT NULL,
  removed_by citext DEFAULT NULL,
  comment text NOT NULL,
  entry_type comment_type NOT NULL DEFAULT 'comment',
  is_persistent boolenum NOT NULL DEFAULT 'n',
  is_sticky boolenum NOT NULL DEFAULT 'n',
  expire_time biguint DEFAULT NULL,
  remove_time biguint DEFAULT NULL,
  has_been_removed boolenum NOT NULL DEFAULT 'n',

  CONSTRAINT pk_comment_history PRIMARY KEY (comment_id)
);

ALTER TABLE comment_history ALTER COLUMN comment_id SET STORAGE PLAIN;
ALTER TABLE comment_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE comment_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE comment_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE comment_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN comment_history.comment_id IS 'comment.id';
COMMENT ON COLUMN comment_history.environment_id IS 'environment.id';
COMMENT ON COLUMN comment_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN comment_history.host_id IS 'host.id';
COMMENT ON COLUMN comment_history.service_id IS 'service.id';

CREATE TABLE flapping_history (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  start_time biguint NOT NULL,
  end_time biguint DEFAULT NULL,
  percent_state_change_start float DEFAULT NULL,
  percent_state_change_end float DEFAULT NULL,
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  CONSTRAINT pk_flapping_history PRIMARY KEY (id)
);

ALTER TABLE flapping_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE flapping_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE flapping_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE flapping_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE flapping_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN flapping_history.id IS 'sha1(environment.id + "Host"|"Service" + host|service.name + start_time)';
COMMENT ON COLUMN flapping_history.environment_id IS 'environment.id';
COMMENT ON COLUMN flapping_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN flapping_history.host_id IS 'host.id';
COMMENT ON COLUMN flapping_history.service_id IS 'service.id';

CREATE TABLE acknowledgement_history (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,

  set_time biguint NOT NULL,
  clear_time biguint DEFAULT NULL,
  author citext DEFAULT NULL,
  cleared_by citext DEFAULT NULL,
  comment text DEFAULT NULL,
  expire_time biguint DEFAULT NULL,
  is_sticky boolenum DEFAULT NULL,
  is_persistent boolenum DEFAULT NULL,

  CONSTRAINT pk_acknowledgement_history PRIMARY KEY (id)
);

ALTER TABLE acknowledgement_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN acknowledgement_history.id IS 'sha1(environment.id + "Host"|"Service" + host|service.name + set_time)';
COMMENT ON COLUMN acknowledgement_history.environment_id IS 'environment.id';
COMMENT ON COLUMN acknowledgement_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN acknowledgement_history.host_id IS 'host.id';
COMMENT ON COLUMN acknowledgement_history.service_id IS 'service.id';
COMMENT ON COLUMN acknowledgement_history.author IS 'NULL if ack_set event happened before Icinga DB history recording';
COMMENT ON COLUMN acknowledgement_history.comment IS 'NULL if ack_set event happened before Icinga DB history recording';
COMMENT ON COLUMN acknowledgement_history.is_sticky IS 'NULL if ack_set event happened before Icinga DB history recording';
COMMENT ON COLUMN acknowledgement_history.is_persistent IS 'NULL if ack_set event happened before Icinga DB history recording';

CREATE TABLE history (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  endpoint_id bytea20 DEFAULT NULL,
  object_type checkable_type NOT NULL DEFAULT 'host',
  host_id bytea20 NOT NULL,
  service_id bytea20 DEFAULT NULL,
  notification_history_id bytea20 DEFAULT NULL,
  state_history_id bytea20 DEFAULT NULL,
  downtime_history_id bytea20 DEFAULT NULL,
  comment_history_id bytea20 DEFAULT NULL,
  flapping_history_id bytea20 DEFAULT NULL,
  acknowledgement_history_id bytea20 DEFAULT NULL,

  event_type history_type NOT NULL DEFAULT 'notification',
  event_time biguint NOT NULL,

  CONSTRAINT pk_history PRIMARY KEY (id),

  CONSTRAINT fk_history_acknowledgement_history FOREIGN KEY (acknowledgement_history_id) REFERENCES acknowledgement_history (id) ON DELETE CASCADE,
  CONSTRAINT fk_history_comment_history FOREIGN KEY (comment_history_id) REFERENCES comment_history (comment_id) ON DELETE CASCADE,
  CONSTRAINT fk_history_downtime_history FOREIGN KEY (downtime_history_id) REFERENCES downtime_history (downtime_id) ON DELETE CASCADE,
  CONSTRAINT fk_history_flapping_history FOREIGN KEY (flapping_history_id) REFERENCES flapping_history (id) ON DELETE CASCADE,
  CONSTRAINT fk_history_notification_history FOREIGN KEY (notification_history_id) REFERENCES notification_history (id) ON DELETE CASCADE,
  CONSTRAINT fk_history_state_history FOREIGN KEY (state_history_id) REFERENCES state_history (id) ON DELETE CASCADE
);

ALTER TABLE history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN notification_history_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN state_history_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN downtime_history_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN comment_history_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN flapping_history_id SET STORAGE PLAIN;
ALTER TABLE history ALTER COLUMN acknowledgement_history_id SET STORAGE PLAIN;

CREATE INDEX idx_history_event_time ON history(event_time);
CREATE INDEX idx_history_acknowledgement ON history(acknowledgement_history_id);
CREATE INDEX idx_history_comment ON history(comment_history_id);
CREATE INDEX idx_history_downtime ON history(downtime_history_id);
CREATE INDEX idx_history_flapping ON history(flapping_history_id);
CREATE INDEX idx_history_notification ON history(notification_history_id);
CREATE INDEX idx_history_state ON history(state_history_id);
CREATE INDEX idx_history_host_service_id ON history(host_id, service_id, event_time);

COMMENT ON COLUMN history.id IS 'sha1(environment.name + event_type + x...) given that sha1(environment.name + x...) = *_history_id';
COMMENT ON COLUMN history.environment_id IS 'environment.id';
COMMENT ON COLUMN history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN history.host_id IS 'host.id';
COMMENT ON COLUMN history.service_id IS 'service.id';
COMMENT ON COLUMN history.notification_history_id IS 'notification_history.id';
COMMENT ON COLUMN history.state_history_id IS 'state_history.id';
COMMENT ON COLUMN history.downtime_history_id IS 'downtime_history.downtime_id';
COMMENT ON COLUMN history.comment_history_id IS 'comment_history.comment_id';
COMMENT ON COLUMN history.flapping_history_id IS 'flapping_history.id';
COMMENT ON COLUMN history.acknowledgement_history_id IS 'acknowledgement_history.id';

COMMENT ON INDEX idx_history_event_time IS 'History filtered/ordered by event_time';
COMMENT ON INDEX idx_history_host_service_id IS 'Host/service history detail filter';

CREATE SEQUENCE icingadb_schema_id_seq;

CREATE TABLE icingadb_schema (
  id uint NOT NULL DEFAULT nextval('icingadb_schema_id_seq'),
  version smalluint NOT NULL,
  timestamp biguint NOT NULL,

  CONSTRAINT pk_icingadb_schema PRIMARY KEY (id)
);

ALTER SEQUENCE icingadb_schema_id_seq OWNED BY icingadb_schema.id;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (1, extract(epoch from now()) * 1000);
