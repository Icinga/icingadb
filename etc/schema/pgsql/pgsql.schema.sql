-- Icinga DB | (c) 2020 Icinga GmbH | GPLv2+

CREATE DOMAIN bin20 AS bytea CONSTRAINT exactly_20_bytes_long CHECK ( VALUE IS NULL OR octet_length(VALUE) = 20 );
CREATE DOMAIN bin16 AS bytea CONSTRAINT exactly_16_bytes_long CHECK ( VALUE IS NULL OR octet_length(VALUE) = 16 );
CREATE DOMAIN bin4 AS bytea CONSTRAINT exactly_4_bytes_long CHECK ( VALUE IS NULL OR octet_length(VALUE) = 4 );

CREATE DOMAIN uint8 AS int8 CONSTRAINT positive CHECK ( VALUE IS NULL OR 0 <= VALUE );
CREATE DOMAIN uint4 AS int8 CONSTRAINT between_0_and_4294967295 CHECK ( VALUE IS NULL OR VALUE BETWEEN 0 AND 4294967295 );
CREATE DOMAIN uint2 AS int4 CONSTRAINT between_0_and_65535 CHECK ( VALUE IS NULL OR VALUE BETWEEN 0 AND 65535 );
CREATE DOMAIN uint1 AS int2 CONSTRAINT between_0_and_255 CHECK ( VALUE IS NULL OR VALUE BETWEEN 0 AND 255 );

CREATE TYPE bool AS ENUM ( 'y', 'n' );
CREATE TYPE acked AS ENUM ( 'y', 'n', 'sticky' );
CREATE TYPE state_type AS ENUM ( 'hard', 'soft' );
CREATE TYPE checkable_type AS ENUM ( 'host', 'service' );
CREATE TYPE comment_type AS ENUM ( 'comment', 'ack' );
CREATE TYPE notification_history_type AS ENUM ( 'downtime_start', 'downtime_end', 'downtime_removed', 'custom', 'acknowledgement', 'problem', 'recovery', 'flapping_start', 'flapping_end' );
CREATE TYPE history_type AS ENUM ( 'notification', 'state_change', 'downtime_start', 'downtime_end', 'comment_add', 'comment_remove', 'flapping_start', 'flapping_end', 'ack_set', 'ack_clear' );

CREATE TABLE host (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,

  address varchar(255) NOT NULL,
  address6 varchar(255) NOT NULL,
  address_bin bin4 DEFAULT NULL,
  address6_bin bin16 DEFAULT NULL,

  checkcommand varchar(255) NOT NULL,
  checkcommand_id bin20 NOT NULL,

  max_check_attempts uint4 NOT NULL,

  check_timeperiod varchar(255) NOT NULL,
  check_timeperiod_id bin20 DEFAULT NULL,

  check_timeout uint4 DEFAULT NULL,
  check_interval uint4 NOT NULL,
  check_retry_interval uint4 NOT NULL,

  active_checks_enabled bool NOT NULL,
  passive_checks_enabled bool NOT NULL,
  event_handler_enabled bool NOT NULL,
  notifications_enabled bool NOT NULL,

  flapping_enabled bool NOT NULL,
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  perfdata_enabled bool NOT NULL,

  eventcommand varchar(255) NOT NULL,
  eventcommand_id bin20 DEFAULT NULL,

  is_volatile bool NOT NULL,

  action_url_id bin20 DEFAULT NULL,
  notes_url_id bin20 DEFAULT NULL,
  notes text NOT NULL,
  icon_image_id bin20 DEFAULT NULL,
  icon_image_alt varchar(32) NOT NULL,

  zone varchar(255) NOT NULL,
  zone_id bin20 DEFAULT NULL,

  command_endpoint varchar(255) NOT NULL,
  command_endpoint_id bin20 DEFAULT NULL,

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
CREATE INDEX idx_host_display_name ON host(LOWER(display_name));
CREATE INDEX idx_host_name ON host(name);

COMMENT ON COLUMN host.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN host.environment_id IS 'sha1(environment.name)';
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
COMMENT ON INDEX idx_host_name IS 'Host list filtered/ordered by name; Host detail filter';

CREATE TABLE hostgroup (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_hostgroup PRIMARY KEY (id)
);

ALTER TABLE hostgroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE hostgroup ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN hostgroup.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN hostgroup.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN hostgroup.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN hostgroup.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN hostgroup.zone_id IS 'zone.id';

CREATE TABLE hostgroup_member (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  host_id bin20 NOT NULL,
  hostgroup_id bin20 NOT NULL,

  CONSTRAINT pk_hostgroup_member PRIMARY KEY (id)
);

ALTER TABLE hostgroup_member ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE hostgroup_member ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_member ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_member ALTER COLUMN hostgroup_id SET STORAGE PLAIN;

CREATE INDEX idx_hostgroup_member_host_id ON hostgroup_member(host_id, hostgroup_id);
CREATE INDEX idx_hostgroup_member_hostgroup_id ON hostgroup_member(hostgroup_id, host_id);

COMMENT ON COLUMN hostgroup_member.id IS 'sha1(environment.name + host_id + hostgroup_id)';
COMMENT ON COLUMN hostgroup_member.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN hostgroup_member.host_id IS 'host.id';
COMMENT ON COLUMN hostgroup_member.hostgroup_id IS 'hostgroup.id';

CREATE TABLE host_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  host_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_host_customvar PRIMARY KEY (id)
);

ALTER TABLE host_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE host_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE host_customvar ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE host_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_host_customvar_host_id ON host_customvar(host_id, customvar_id);
CREATE INDEX idx_host_customvar_customvar_id ON host_customvar(customvar_id, host_id);

COMMENT ON COLUMN host_customvar.id IS 'sha1(environment.name + host_id + customvar_id)';
COMMENT ON COLUMN host_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN host_customvar.host_id IS 'host.id';
COMMENT ON COLUMN host_customvar.customvar_id IS 'customvar.id';

CREATE TABLE hostgroup_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  hostgroup_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_hostgroup_customvar PRIMARY KEY (id)
);

ALTER TABLE hostgroup_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE hostgroup_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_customvar ALTER COLUMN hostgroup_id SET STORAGE PLAIN;
ALTER TABLE hostgroup_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN hostgroup_customvar.id IS 'sha1(environment.name + hostgroup_id + customvar_id)';
COMMENT ON COLUMN hostgroup_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN hostgroup_customvar.hostgroup_id IS 'hostgroup.id';
COMMENT ON COLUMN hostgroup_customvar.customvar_id IS 'customvar.id';

CREATE TABLE host_state (
  host_id bin20 NOT NULL,
  environment_id bin20 NOT NULL,

  state_type state_type NOT NULL,
  soft_state uint1 NOT NULL,
  hard_state uint1 NOT NULL,
  previous_hard_state uint1 NOT NULL,
  attempt uint1 NOT NULL,
  severity uint2 NOT NULL,

  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  performance_data text DEFAULT NULL,
  check_commandline text DEFAULT NULL,

  is_problem bool NOT NULL,
  is_handled bool NOT NULL,
  is_reachable bool NOT NULL,
  is_flapping bool NOT NULL,
  is_overdue bool NOT NULL,

  is_acknowledged acked NOT NULL,
  acknowledgement_comment_id bin20 DEFAULT NULL,

  in_downtime bool NOT NULL,

  execution_time uint4 DEFAULT NULL,
  latency uint4 DEFAULT NULL,
  timeout uint4 DEFAULT NULL,
  check_source text DEFAULT NULL,

  last_update uint8 DEFAULT NULL,
  last_state_change uint8 NOT NULL,
  next_check uint8 NOT NULL,
  next_update uint8 NOT NULL,

  CONSTRAINT pk_host_state PRIMARY KEY (host_id)
);

ALTER TABLE host_state ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE host_state ALTER COLUMN acknowledgement_comment_id SET STORAGE PLAIN;

COMMENT ON COLUMN host_state.host_id IS 'host.id';
COMMENT ON COLUMN host_state.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN host_state.acknowledgement_comment_id IS 'comment.id';

CREATE TABLE service (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,
  host_id bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,

  checkcommand varchar(255) NOT NULL,
  checkcommand_id bin20 NOT NULL,

  max_check_attempts uint4 NOT NULL,

  check_timeperiod varchar(255) NOT NULL,
  check_timeperiod_id bin20 DEFAULT NULL,

  check_timeout uint4 DEFAULT NULL,
  check_interval uint4 NOT NULL,
  check_retry_interval uint4 NOT NULL,

  active_checks_enabled bool NOT NULL,
  passive_checks_enabled bool NOT NULL,
  event_handler_enabled bool NOT NULL,
  notifications_enabled bool NOT NULL,

  flapping_enabled bool NOT NULL,
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  perfdata_enabled bool NOT NULL,

  eventcommand varchar(255) NOT NULL,
  eventcommand_id bin20 DEFAULT NULL,

  is_volatile bool NOT NULL,

  action_url_id bin20 DEFAULT NULL,
  notes_url_id bin20 DEFAULT NULL,
  notes text NOT NULL,
  icon_image_id bin20 DEFAULT NULL,
  icon_image_alt varchar(32) NOT NULL,

  zone varchar(255) NOT NULL,
  zone_id bin20 DEFAULT NULL,

  command_endpoint varchar(255) NOT NULL,
  command_endpoint_id bin20 DEFAULT NULL,

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

CREATE INDEX idx_service_display_name ON service(LOWER(display_name));
CREATE INDEX idx_service_host_id ON service(host_id, display_name);
CREATE INDEX idx_service_name ON service(name);

COMMENT ON COLUMN service.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN service.environment_id IS 'sha1(environment.name)';
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
COMMENT ON INDEX idx_service_name IS 'Service list filtered/ordered by name; Service detail filter';

CREATE TABLE servicegroup (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_servicegroup PRIMARY KEY (id)
);

ALTER TABLE servicegroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE servicegroup ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN servicegroup.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN servicegroup.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN servicegroup.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN servicegroup.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN servicegroup.zone_id IS 'zone.id';

CREATE TABLE servicegroup_member (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  service_id bin20 NOT NULL,
  servicegroup_id bin20 NOT NULL,

  CONSTRAINT pk_servicegroup_member PRIMARY KEY (id)
);

ALTER TABLE servicegroup_member ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE servicegroup_member ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_member ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_member ALTER COLUMN servicegroup_id SET STORAGE PLAIN;

CREATE INDEX idx_servicegroup_member_service_id ON servicegroup_member(service_id, servicegroup_id);
CREATE INDEX idx_servicegroup_member_servicegroup_id ON servicegroup_member(servicegroup_id, service_id);

COMMENT ON COLUMN servicegroup_member.id IS 'sha1(environment.name + servicegroup_id + service_id)';
COMMENT ON COLUMN servicegroup_member.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN servicegroup_member.service_id IS 'service.id';
COMMENT ON COLUMN servicegroup_member.servicegroup_id IS 'servicegroup.id';

CREATE TABLE service_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  service_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_service_customvar PRIMARY KEY (id)
);

ALTER TABLE service_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE service_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE service_customvar ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE service_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

CREATE INDEX idx_service_customvar_service_id ON service_customvar(service_id, customvar_id);
CREATE INDEX idx_service_customvar_customvar_id ON service_customvar(customvar_id, service_id);

COMMENT ON COLUMN service_customvar.id IS 'sha1(environment.name + service_id + customvar_id)';
COMMENT ON COLUMN service_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN service_customvar.service_id IS 'service.id';
COMMENT ON COLUMN service_customvar.customvar_id IS 'customvar.id';

CREATE TABLE servicegroup_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  servicegroup_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_servicegroup_customvar PRIMARY KEY (id)
);

ALTER TABLE servicegroup_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE servicegroup_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_customvar ALTER COLUMN servicegroup_id SET STORAGE PLAIN;
ALTER TABLE servicegroup_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN servicegroup_customvar.id IS 'sha1(environment.name + servicegroup_id + customvar_id)';
COMMENT ON COLUMN servicegroup_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN servicegroup_customvar.servicegroup_id IS 'servicegroup.id';
COMMENT ON COLUMN servicegroup_customvar.customvar_id IS 'customvar.id';

CREATE TABLE service_state (
  service_id bin20 NOT NULL,
  environment_id bin20 NOT NULL,

  state_type state_type NOT NULL,
  soft_state uint1 NOT NULL,
  hard_state uint1 NOT NULL,
  previous_hard_state uint1 NOT NULL,
  attempt uint1 NOT NULL,
  severity uint2 NOT NULL,

  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  performance_data text DEFAULT NULL,
  check_commandline text DEFAULT NULL,

  is_problem bool NOT NULL,
  is_handled bool NOT NULL,
  is_reachable bool NOT NULL,
  is_flapping bool NOT NULL,
  is_overdue bool NOT NULL,

  is_acknowledged acked NOT NULL,
  acknowledgement_comment_id bin20 DEFAULT NULL,

  in_downtime bool NOT NULL,

  execution_time uint4 DEFAULT NULL,
  latency uint4 DEFAULT NULL,
  timeout uint4 DEFAULT NULL,
  check_source text DEFAULT NULL,

  last_update uint8 DEFAULT NULL,
  last_state_change uint8 NOT NULL,
  next_check uint8 NOT NULL,
  next_update uint8 NOT NULL,

  CONSTRAINT pk_service_state PRIMARY KEY (service_id)
);

ALTER TABLE service_state ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE service_state ALTER COLUMN acknowledgement_comment_id SET STORAGE PLAIN;

COMMENT ON COLUMN service_state.service_id IS 'service.id';
COMMENT ON COLUMN service_state.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN service_state.acknowledgement_comment_id IS 'comment.id';

CREATE TABLE endpoint (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,

  zone_id bin20 NOT NULL,

  CONSTRAINT pk_endpoint PRIMARY KEY (id)
);

ALTER TABLE endpoint ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE endpoint ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN endpoint.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN endpoint.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN endpoint.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN endpoint.zone_id IS 'zone.id';

CREATE TABLE environment (
  id bin20 NOT NULL,
  name varchar(255) NOT NULL,

  CONSTRAINT pk_environment PRIMARY KEY (id)
);

ALTER TABLE environment ALTER COLUMN id SET STORAGE PLAIN;

COMMENT ON COLUMN environment.id IS 'sha1(name)';

CREATE TABLE icingadb_instance (
  id bin16 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  heartbeat uint8 NOT NULL,
  responsible bool NOT NULL,

  icinga2_version varchar(255) NOT NULL,
  icinga2_start_time uint8 NOT NULL,
  icinga2_notifications_enabled bool NOT NULL,
  icinga2_active_service_checks_enabled bool NOT NULL,
  icinga2_active_host_checks_enabled bool NOT NULL,
  icinga2_event_handlers_enabled bool NOT NULL,
  icinga2_flap_detection_enabled bool NOT NULL,
  icinga2_performance_data_enabled bool NOT NULL,

  CONSTRAINT pk_icingadb_instance PRIMARY KEY (id)
);

ALTER TABLE icingadb_instance ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE icingadb_instance ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE icingadb_instance ALTER COLUMN endpoint_id SET STORAGE PLAIN;

COMMENT ON COLUMN icingadb_instance.id IS 'UUIDv4';
COMMENT ON COLUMN icingadb_instance.environment_id IS 'environment.id';
COMMENT ON COLUMN icingadb_instance.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN icingadb_instance.heartbeat IS '*nix timestamp';

CREATE SEQUENCE icingadb_schema_id_seq;

CREATE TABLE icingadb_schema (
  id uint4 NOT NULL DEFAULT nextval('icingadb_schema_id_seq'),
  version uint2 NOT NULL,
  timestamp uint8 NOT NULL,

  CONSTRAINT pk_icingadb_schema PRIMARY KEY (id)
);

ALTER SEQUENCE icingadb_schema_id_seq OWNED BY icingadb_schema.id;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (1, extract(epoch from now()) * 1000);

CREATE TABLE checkcommand (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  zone_id bin20 DEFAULT NULL,

  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  command text NOT NULL,
  timeout uint4 NOT NULL,

  CONSTRAINT pk_checkcommand PRIMARY KEY (id)
);

ALTER TABLE checkcommand ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE checkcommand ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand.id IS 'sha1(environment.name + type + name)';
COMMENT ON COLUMN checkcommand.environment_id IS 'env.id';
COMMENT ON COLUMN checkcommand.zone_id IS 'zone.id';
COMMENT ON COLUMN checkcommand.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN checkcommand.properties_checksum IS 'sha1(all properties)';

CREATE TABLE checkcommand_argument (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  argument_key varchar(64) NOT NULL,

  properties_checksum bin20 NOT NULL,

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) DEFAULT NULL,
  repeat_key bool NOT NULL,
  required bool NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key bool NOT NULL,

  CONSTRAINT pk_checkcommand_argument PRIMARY KEY (id)
);

ALTER TABLE checkcommand_argument ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN argument_key SET STORAGE PLAIN;
ALTER TABLE checkcommand_argument ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand_argument.id IS 'sha1(environment.name + command_id + argument_key)';
COMMENT ON COLUMN checkcommand_argument.environment_id IS 'env.id';
COMMENT ON COLUMN checkcommand_argument.command_id IS 'command.id';
COMMENT ON COLUMN checkcommand_argument.properties_checksum IS 'sha1(all properties)';

CREATE TABLE checkcommand_envvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  envvar_key varchar(64) NOT NULL,

  properties_checksum bin20 NOT NULL,

  envvar_value text NOT NULL,

  CONSTRAINT pk_checkcommand_envvar PRIMARY KEY (id)
);

ALTER TABLE checkcommand_envvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand_envvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_envvar ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_envvar ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand_envvar.id IS 'sha1(environment.name + command_id + envvar_key)';
COMMENT ON COLUMN checkcommand_envvar.environment_id IS 'env.id';
COMMENT ON COLUMN checkcommand_envvar.command_id IS 'command.id';
COMMENT ON COLUMN checkcommand_envvar.properties_checksum IS 'sha1(all properties)';

CREATE TABLE checkcommand_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,

  command_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_checkcommand_customvar PRIMARY KEY (id)
);

ALTER TABLE checkcommand_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE checkcommand_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_customvar ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE checkcommand_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN checkcommand_customvar.id IS 'sha1(environment.name + command_id + customvar_id)';
COMMENT ON COLUMN checkcommand_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN checkcommand_customvar.command_id IS 'command.id';
COMMENT ON COLUMN checkcommand_customvar.customvar_id IS 'customvar.id';

CREATE TABLE eventcommand (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  zone_id bin20 DEFAULT NULL,

  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  command text NOT NULL,
  timeout uint2 NOT NULL,

  CONSTRAINT pk_eventcommand PRIMARY KEY (id)
);

ALTER TABLE eventcommand ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE eventcommand ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand.id IS 'sha1(environment.name + type + name)';
COMMENT ON COLUMN eventcommand.environment_id IS 'env.id';
COMMENT ON COLUMN eventcommand.zone_id IS 'zone.id';
COMMENT ON COLUMN eventcommand.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN eventcommand.properties_checksum IS 'sha1(all properties)';

CREATE TABLE eventcommand_argument (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  argument_key varchar(64) NOT NULL,

  properties_checksum bin20 NOT NULL,

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) DEFAULT NULL,
  repeat_key bool NOT NULL,
  required bool NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key bool NOT NULL,

  CONSTRAINT pk_eventcommand_argument PRIMARY KEY (id)
);

ALTER TABLE eventcommand_argument ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand_argument ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_argument ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_argument ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand_argument.id IS 'sha1(environment.name + command_id + argument_key)';
COMMENT ON COLUMN eventcommand_argument.environment_id IS 'env.id';
COMMENT ON COLUMN eventcommand_argument.command_id IS 'command.id';
COMMENT ON COLUMN eventcommand_argument.properties_checksum IS 'sha1(all properties)';

CREATE TABLE eventcommand_envvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  envvar_key varchar(64) NOT NULL,

  properties_checksum bin20 NOT NULL,

  envvar_value text NOT NULL,

  CONSTRAINT pk_eventcommand_envvar PRIMARY KEY (id)
);

ALTER TABLE eventcommand_envvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand_envvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_envvar ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_envvar ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand_envvar.id IS 'sha1(environment.name + command_id + envvar_key)';
COMMENT ON COLUMN eventcommand_envvar.environment_id IS 'env.id';
COMMENT ON COLUMN eventcommand_envvar.command_id IS 'command.id';
COMMENT ON COLUMN eventcommand_envvar.properties_checksum IS 'sha1(all properties)';

CREATE TABLE eventcommand_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_eventcommand_customvar PRIMARY KEY (id)
);

ALTER TABLE eventcommand_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE eventcommand_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_customvar ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE eventcommand_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN eventcommand_customvar.id IS 'sha1(environment.name + command_id + customvar_id)';
COMMENT ON COLUMN eventcommand_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN eventcommand_customvar.command_id IS 'command.id';
COMMENT ON COLUMN eventcommand_customvar.customvar_id IS 'customvar.id';

CREATE TABLE notificationcommand (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  zone_id bin20 DEFAULT NULL,

  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  command text NOT NULL,
  timeout uint2 NOT NULL,

  CONSTRAINT pk_notificationcommand PRIMARY KEY (id)
);

ALTER TABLE notificationcommand ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN zone_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE notificationcommand ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand.id IS 'sha1(environment.name + type + name)';
COMMENT ON COLUMN notificationcommand.environment_id IS 'env.id';
COMMENT ON COLUMN notificationcommand.zone_id IS 'zone.id';
COMMENT ON COLUMN notificationcommand.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN notificationcommand.properties_checksum IS 'sha1(all properties)';

CREATE TABLE notificationcommand_argument (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  argument_key varchar(64) NOT NULL,

  properties_checksum bin20 NOT NULL,

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) DEFAULT NULL,
  repeat_key bool NOT NULL,
  required bool NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key bool NOT NULL,

  CONSTRAINT pk_notificationcommand_argument PRIMARY KEY (id)
);

ALTER TABLE notificationcommand_argument ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_argument ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_argument ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_argument ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand_argument.id IS 'sha1(environment.name + command_id + argument_key)';
COMMENT ON COLUMN notificationcommand_argument.environment_id IS 'env.id';
COMMENT ON COLUMN notificationcommand_argument.command_id IS 'command.id';
COMMENT ON COLUMN notificationcommand_argument.properties_checksum IS 'sha1(all properties)';

CREATE TABLE notificationcommand_envvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  envvar_key varchar(64) NOT NULL,

  properties_checksum bin20 NOT NULL,

  envvar_value text NOT NULL,

  CONSTRAINT pk_notificationcommand_envvar PRIMARY KEY (id)
);

ALTER TABLE notificationcommand_envvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_envvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_envvar ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_envvar ALTER COLUMN properties_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand_envvar.id IS 'sha1(environment.name + command_id + envvar_key)';
COMMENT ON COLUMN notificationcommand_envvar.environment_id IS 'env.id';
COMMENT ON COLUMN notificationcommand_envvar.command_id IS 'command.id';
COMMENT ON COLUMN notificationcommand_envvar.properties_checksum IS 'sha1(all properties)';

CREATE TABLE notificationcommand_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  command_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_notificationcommand_customvar PRIMARY KEY (id)
);

ALTER TABLE notificationcommand_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_customvar ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE notificationcommand_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN notificationcommand_customvar.id IS 'sha1(environment.name + command_id + customvar_id)';
COMMENT ON COLUMN notificationcommand_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN notificationcommand_customvar.command_id IS 'command.id';
COMMENT ON COLUMN notificationcommand_customvar.customvar_id IS 'customvar.id';

CREATE TABLE comment (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,

  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL,
  text text NOT NULL,
  entry_type comment_type NOT NULL,
  entry_time uint8 NOT NULL,
  is_persistent bool NOT NULL,
  is_sticky bool NOT NULL,
  expire_time uint8 DEFAULT NULL,

  zone_id bin20 DEFAULT NULL,

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

COMMENT ON COLUMN comment.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN comment.environment_id IS 'environment.id';
COMMENT ON COLUMN comment.host_id IS 'host.id';
COMMENT ON COLUMN comment.service_id IS 'service.id';
COMMENT ON COLUMN comment.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN comment.zone_id IS 'zone.id';

COMMENT ON INDEX idx_comment_name IS 'Comment detail filter';
COMMENT ON INDEX idx_comment_entry_time IS 'Comment list fileted/ordered by entry_time';

CREATE TABLE downtime (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,

  triggered_by_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL,
  comment text NOT NULL,
  entry_time uint8 NOT NULL,
  scheduled_start_time uint8 NOT NULL,
  scheduled_end_time uint8 NOT NULL,
  flexible_duration uint8 NOT NULL,
  is_flexible bool NOT NULL,

  is_in_effect bool NOT NULL,
  start_time uint8 DEFAULT NULL,
  end_time uint8 DEFAULT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_downtime PRIMARY KEY (id)
);

ALTER TABLE downtime ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN triggered_by_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE downtime ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_downtime_is_in_effect ON downtime(is_in_effect, start_time);
CREATE INDEX idx_downtime_name ON downtime(name);

COMMENT ON COLUMN downtime.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN downtime.environment_id IS 'environment.id';
COMMENT ON COLUMN downtime.triggered_by_id IS 'downtime.id';
COMMENT ON COLUMN downtime.host_id IS 'host.id';
COMMENT ON COLUMN downtime.service_id IS 'service.id';
COMMENT ON COLUMN downtime.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN downtime.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN downtime.start_time IS 'Time when the host went into a problem state during the downtimes timeframe';
COMMENT ON COLUMN downtime.end_time IS 'Problem state assumed: scheduled_end_time if fixed, start_time + flexible_duration otherwise';
COMMENT ON COLUMN downtime.zone_id IS 'zone.id';

COMMENT ON INDEX idx_downtime_is_in_effect IS 'Downtime list filtered/ordered by severity';
COMMENT ON INDEX idx_downtime_name IS 'Downtime detail filter';

CREATE TABLE notification (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,

  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,
  command_id bin20 NOT NULL,

  times_begin uint4 DEFAULT NULL,
  times_end uint4 DEFAULT NULL,
  notification_interval uint4 NOT NULL,
  timeperiod_id bin20 DEFAULT NULL,

  states uint1 NOT NULL,
  types uint2 NOT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_notification PRIMARY KEY (id)
);

ALTER TABLE notification ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN command_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE notification ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_host_id ON notification(host_id);
CREATE INDEX idx_service_id ON notification(service_id);

COMMENT ON COLUMN notification.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN notification.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN notification.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN notification.host_id IS 'host.id';
COMMENT ON COLUMN notification.service_id IS 'service.id';
COMMENT ON COLUMN notification.command_id IS 'command.id';
COMMENT ON COLUMN notification.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN notification.zone_id IS 'zone.id';

CREATE TABLE notification_user (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  notification_id bin20 NOT NULL,
  user_id bin20 NOT NULL,

  CONSTRAINT pk_notification_user PRIMARY KEY (id)
);

ALTER TABLE notification_user ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_user ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_user ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_user ALTER COLUMN user_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_user_user_id ON notification_user(user_id, notification_id);
CREATE INDEX idx_notification_user_notification_id ON notification_user(notification_id, user_id);

COMMENT ON COLUMN notification_user.id IS 'sha1(environment.name + notification_id + user_id)';
COMMENT ON COLUMN notification_user.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_user.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_user.user_id IS 'user.id';

CREATE TABLE notification_usergroup (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  notification_id bin20 NOT NULL,
  usergroup_id bin20 NOT NULL,

  CONSTRAINT pk_notification_usergroup PRIMARY KEY (id)
);

ALTER TABLE notification_usergroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_usergroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_usergroup ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_usergroup ALTER COLUMN usergroup_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_usergroup_usergroup_id ON notification_usergroup(usergroup_id, notification_id);
CREATE INDEX idx_notification_usergroup_notification_id ON notification_usergroup(notification_id, usergroup_id);

COMMENT ON COLUMN notification_usergroup.id IS 'sha1(environment.name + notification_id + usergroup_id)';
COMMENT ON COLUMN notification_usergroup.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_usergroup.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_usergroup.usergroup_id IS 'usergroup.id';

CREATE TABLE notification_recipient (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  notification_id bin20 NOT NULL,
  user_id bin20 NULL,
  usergroup_id bin20 NULL,

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

COMMENT ON COLUMN notification_recipient.id IS 'sha1(environment.name + notification_id + (user_id | usergroup_id))';
COMMENT ON COLUMN notification_recipient.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_recipient.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_recipient.user_id IS 'user.id';
COMMENT ON COLUMN notification_recipient.usergroup_id IS 'usergroup.id';

CREATE TABLE notification_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  notification_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_notification_customvar PRIMARY KEY (id)
);

ALTER TABLE notification_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_customvar ALTER COLUMN notification_id SET STORAGE PLAIN;
ALTER TABLE notification_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN notification_customvar.id IS 'sha1(environment.name + notification_id + customvar_id)';
COMMENT ON COLUMN notification_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN notification_customvar.notification_id IS 'notification.id';
COMMENT ON COLUMN notification_customvar.customvar_id IS 'customvar.id';

CREATE TABLE icon_image (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  icon_image text NOT NULL,

  CONSTRAINT pk_icon_image PRIMARY KEY (environment_id, id)
);

ALTER TABLE icon_image ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE icon_image ALTER COLUMN environment_id SET STORAGE PLAIN;

CREATE INDEX idx_icon_image ON icon_image(LOWER(icon_image));

COMMENT ON COLUMN icon_image.id IS 'sha1(icon_image)';
COMMENT ON COLUMN icon_image.environment_id IS 'sha1(environment.name)';

CREATE TABLE action_url (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  action_url text NOT NULL,

  CONSTRAINT pk_action_url PRIMARY KEY (environment_id, id)
);

ALTER TABLE action_url ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE action_url ALTER COLUMN environment_id SET STORAGE PLAIN;

CREATE INDEX idx_action_url ON action_url(LOWER(action_url));

COMMENT ON COLUMN action_url.id IS 'sha1(action_url)';
COMMENT ON COLUMN action_url.environment_id IS 'sha1(environment.name)';

CREATE TABLE notes_url (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  notes_url text NOT NULL,

  CONSTRAINT pk_notes_url PRIMARY KEY (environment_id, id)
);

ALTER TABLE notes_url ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notes_url ALTER COLUMN environment_id SET STORAGE PLAIN;

CREATE INDEX idx_notes_url ON notes_url(LOWER(notes_url));

COMMENT ON COLUMN notes_url.id IS 'sha1(notes_url)';
COMMENT ON COLUMN notes_url.environment_id IS 'sha1(environment.name)';

CREATE TABLE timeperiod (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,

  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,
  prefer_includes bool NOT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_timeperiod PRIMARY KEY (id)
);

ALTER TABLE timeperiod ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE timeperiod ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod.id IS 'sha1(env.name + name)';
COMMENT ON COLUMN timeperiod.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN timeperiod.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN timeperiod.zone_id IS 'zone.id';

CREATE TABLE timeperiod_range (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  timeperiod_id bin20 NOT NULL,
  range_key varchar(255) NOT NULL,

  range_value varchar(255) NOT NULL,

  CONSTRAINT pk_timeperiod_range PRIMARY KEY (id)
);

ALTER TABLE timeperiod_range ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_range ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_range ALTER COLUMN timeperiod_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_range.id IS 'sha1(environment.name + range_id + timeperiod_id)';
COMMENT ON COLUMN timeperiod_range.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod_range.timeperiod_id IS 'timeperiod.id';

CREATE TABLE timeperiod_override_include (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  timeperiod_id bin20 NOT NULL,
  override_id bin20 NOT NULL,

  CONSTRAINT pk_timeperiod_override_include PRIMARY KEY (id)
);

ALTER TABLE timeperiod_override_include ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_include ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_include ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_include ALTER COLUMN override_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_override_include.id IS 'sha1(environment.name + include_id + timeperiod_id)';
COMMENT ON COLUMN timeperiod_override_include.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod_override_include.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN timeperiod_override_include.override_id IS 'timeperiod.id';

CREATE TABLE timeperiod_override_exclude (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  timeperiod_id bin20 NOT NULL,
  override_id bin20 NOT NULL,

  CONSTRAINT pk_timeperiod_override_exclude PRIMARY KEY (id)
);

ALTER TABLE timeperiod_override_exclude ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_exclude ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_exclude ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_override_exclude ALTER COLUMN override_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_override_exclude.id IS 'sha1(environment.name + exclude_id + timeperiod_id)';
COMMENT ON COLUMN timeperiod_override_exclude.environment_id IS 'env.id';
COMMENT ON COLUMN timeperiod_override_exclude.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN timeperiod_override_exclude.override_id IS 'timeperiod.id';

CREATE TABLE timeperiod_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  timeperiod_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_timeperiod_customvar PRIMARY KEY (id)
);

ALTER TABLE timeperiod_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE timeperiod_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_customvar ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE timeperiod_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN timeperiod_customvar.id IS 'sha1(environment.name + timeperiod_id + customvar_id)';
COMMENT ON COLUMN timeperiod_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN timeperiod_customvar.timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN timeperiod_customvar.customvar_id IS 'customvar.id';

CREATE TABLE customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  value text NOT NULL,

  CONSTRAINT pk_customvar PRIMARY KEY (id)
);

ALTER TABLE customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE customvar ALTER COLUMN name_checksum SET STORAGE PLAIN;

COMMENT ON COLUMN customvar.id IS 'sha1(environment.name + name + value)';
COMMENT ON COLUMN customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN customvar.name_checksum IS 'sha1(name)';

CREATE TABLE customvar_flat (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,
  flatname_checksum bin20 NOT NULL,

  flatname varchar(512) NOT NULL,
  flatvalue text NOT NULL,

  CONSTRAINT pk_customvar_flat PRIMARY KEY (id)
);

ALTER TABLE customvar_flat ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE customvar_flat ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE customvar_flat ALTER COLUMN customvar_id SET STORAGE PLAIN;
ALTER TABLE customvar_flat ALTER COLUMN flatname_checksum SET STORAGE PLAIN;

CREATE INDEX idx_customvar_flat_customvar_id ON customvar_flat(customvar_id);

COMMENT ON COLUMN customvar_flat.id IS 'sha1(environment.name + flatname + flatvalue)';
COMMENT ON COLUMN customvar_flat.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN customvar_flat.customvar_id IS 'sha1(customvar.id)';
COMMENT ON COLUMN customvar_flat.flatname_checksum IS 'sha1(flatname after conversion)';
COMMENT ON COLUMN customvar_flat.flatname IS 'Path converted with `.` and `[ ]`';

CREATE TABLE "user" (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,

  email varchar(255) NOT NULL,
  pager varchar(255) NOT NULL,

  notifications_enabled bool NOT NULL,

  timeperiod_id bin20 DEFAULT NULL,

  states uint1 NOT NULL,
  types uint2 NOT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_user PRIMARY KEY (id)
);

ALTER TABLE "user" ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN timeperiod_id SET STORAGE PLAIN;
ALTER TABLE "user" ALTER COLUMN zone_id SET STORAGE PLAIN;

CREATE INDEX idx_user_display_name ON "user"(LOWER(display_name));

COMMENT ON COLUMN "user".id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN "user".environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN "user".name_checksum IS 'sha1(name)';
COMMENT ON COLUMN "user".properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN "user".timeperiod_id IS 'timeperiod.id';
COMMENT ON COLUMN "user".zone_id IS 'zone.id';

COMMENT ON INDEX idx_user_display_name IS 'User list filtered/ordered by display_name';

CREATE TABLE usergroup (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,
  display_name varchar(255) NOT NULL,

  zone_id bin20 DEFAULT NULL,

  CONSTRAINT pk_usergroup PRIMARY KEY (id)
);

ALTER TABLE usergroup ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE usergroup ALTER COLUMN zone_id SET STORAGE PLAIN;

COMMENT ON COLUMN usergroup.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN usergroup.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN usergroup.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN usergroup.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN usergroup.zone_id IS 'zone.id';

CREATE TABLE usergroup_member (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  user_id bin20 NOT NULL,
  usergroup_id bin20 NOT NULL,

  CONSTRAINT pk_usergroup_member PRIMARY KEY (id)
);

ALTER TABLE usergroup_member ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE usergroup_member ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE usergroup_member ALTER COLUMN user_id SET STORAGE PLAIN;
ALTER TABLE usergroup_member ALTER COLUMN usergroup_id SET STORAGE PLAIN;

CREATE INDEX idx_usergroup_member_user_id ON usergroup_member(user_id, usergroup_id);
CREATE INDEX idx_usergroup_member_usergroup_id ON usergroup_member(usergroup_id, user_id);

COMMENT ON COLUMN usergroup_member.id IS 'sha1(environment.name + usergroup_id + user_id)';
COMMENT ON COLUMN usergroup_member.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN usergroup_member.user_id IS 'user.id';
COMMENT ON COLUMN usergroup_member.usergroup_id IS 'usergroup.id';

CREATE TABLE user_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  user_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_user_customvar PRIMARY KEY (id)
);

ALTER TABLE user_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE user_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE user_customvar ALTER COLUMN user_id SET STORAGE PLAIN;
ALTER TABLE user_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN user_customvar.id IS 'sha1(environment.name + user_id + customvar_id)';
COMMENT ON COLUMN user_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN user_customvar.user_id IS 'user.id';
COMMENT ON COLUMN user_customvar.customvar_id IS 'customvar.id';

CREATE TABLE usergroup_customvar (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  usergroup_id bin20 NOT NULL,
  customvar_id bin20 NOT NULL,

  CONSTRAINT pk_usergroup_customvar PRIMARY KEY (id)
);

ALTER TABLE usergroup_customvar ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE usergroup_customvar ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE usergroup_customvar ALTER COLUMN usergroup_id SET STORAGE PLAIN;
ALTER TABLE usergroup_customvar ALTER COLUMN customvar_id SET STORAGE PLAIN;

COMMENT ON COLUMN usergroup_customvar.id IS 'sha1(environment.name + usergroup_id + customvar_id)';
COMMENT ON COLUMN usergroup_customvar.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN usergroup_customvar.usergroup_id IS 'usergroup.id';
COMMENT ON COLUMN usergroup_customvar.customvar_id IS 'customvar.id';

CREATE TABLE zone (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  name_checksum bin20 NOT NULL,
  properties_checksum bin20 NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) NOT NULL,

  is_global bool NOT NULL,
  parent_id bin20 DEFAULT NULL,

  depth uint1 NOT NULL,

  CONSTRAINT pk_zone PRIMARY KEY (id)
);

ALTER TABLE zone ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN name_checksum SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN properties_checksum SET STORAGE PLAIN;
ALTER TABLE zone ALTER COLUMN parent_id SET STORAGE PLAIN;

CREATE INDEX idx_parent_id ON zone(parent_id);
CREATE UNIQUE INDEX idx_environment_id_id ON zone(environment_id, id);

COMMENT ON COLUMN zone.id IS 'sha1(environment.name + name)';
COMMENT ON COLUMN zone.environment_id IS 'sha1(environment.name)';
COMMENT ON COLUMN zone.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN zone.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN zone.parent_id IS 'zone.id';

CREATE TABLE notification_history (
  id bin16 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,
  notification_id bin20 NOT NULL,

  type notification_history_type NOT NULL,
  send_time uint8 NOT NULL,
  state uint1 NOT NULL,
  previous_hard_state uint1 NOT NULL,
  author text NOT NULL,
  "text" text NOT NULL,
  users_notified uint2 NOT NULL,

  CONSTRAINT pk_notification_history PRIMARY KEY (id)
);

ALTER TABLE notification_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE notification_history ALTER COLUMN notification_id SET STORAGE PLAIN;

CREATE INDEX idx_notification_history_event_time ON notification_history(send_time DESC);

COMMENT ON COLUMN notification_history.id IS 'UUID';
COMMENT ON COLUMN notification_history.environment_id IS 'environment.id';
COMMENT ON COLUMN notification_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN notification_history.host_id IS 'host.id';
COMMENT ON COLUMN notification_history.service_id IS 'service.id';
COMMENT ON COLUMN notification_history.notification_id IS 'notification.id';

COMMENT ON INDEX idx_notification_history_event_time IS 'Notification list filtered/ordered by entry_time';

CREATE TABLE user_notification_history (
  id bin16 NOT NULL,
  environment_id bin20 NOT NULL,
  notification_history_id bin16 NOT NULL,
  user_id bin20 NOT NULL,

  CONSTRAINT pk_user_notification_history PRIMARY KEY (id)
);

ALTER TABLE user_notification_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE user_notification_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE user_notification_history ALTER COLUMN notification_history_id SET STORAGE PLAIN;
ALTER TABLE user_notification_history ALTER COLUMN user_id SET STORAGE PLAIN;

COMMENT ON COLUMN user_notification_history.id IS 'UUID';
COMMENT ON COLUMN user_notification_history.environment_id IS 'environment.id';
COMMENT ON COLUMN user_notification_history.notification_history_id IS 'UUID notification_history.id';
COMMENT ON COLUMN user_notification_history.user_id IS 'user.id';

CREATE TABLE state_history (
  id bin16 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  event_time uint8 NOT NULL,
  state_type state_type NOT NULL,
  soft_state uint1 NOT NULL,
  hard_state uint1 NOT NULL,
  previous_soft_state uint1 NOT NULL,
  previous_hard_state uint1 NOT NULL,
  attempt uint1 NOT NULL,
  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  max_check_attempts uint4 NOT NULL,
  check_source text DEFAULT NULL,

  CONSTRAINT pk_state_history PRIMARY KEY (id)
);

ALTER TABLE state_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE state_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN state_history.id IS 'UUID';
COMMENT ON COLUMN state_history.environment_id IS 'environment.id';
COMMENT ON COLUMN state_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN state_history.host_id IS 'host.id';
COMMENT ON COLUMN state_history.service_id IS 'service.id';

CREATE TABLE downtime_history (
  downtime_id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  triggered_by_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  entry_time uint8 NOT NULL,
  author varchar(255) NOT NULL,
  cancelled_by varchar(255) DEFAULT NULL,
  comment text NOT NULL,
  is_flexible bool NOT NULL,
  flexible_duration uint8 NOT NULL,
  scheduled_start_time uint8 NOT NULL,
  scheduled_end_time uint8 NOT NULL,
  start_time uint8 NOT NULL,
  end_time uint8 NOT NULL,
  has_been_cancelled bool NOT NULL,
  trigger_time uint8 NOT NULL,
  cancel_time uint8 DEFAULT NULL,

  CONSTRAINT pk_downtime_history PRIMARY KEY (downtime_id)
);

ALTER TABLE downtime_history ALTER COLUMN downtime_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN triggered_by_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE downtime_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN downtime_history.downtime_id IS 'downtime.id';
COMMENT ON COLUMN downtime_history.environment_id IS 'environment.id';
COMMENT ON COLUMN downtime_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN downtime_history.triggered_by_id IS 'downtime.id';
COMMENT ON COLUMN downtime_history.host_id IS 'host.id';
COMMENT ON COLUMN downtime_history.service_id IS 'service.id';
COMMENT ON COLUMN downtime_history.start_time IS 'Time when the host went into a problem state during the downtimes timeframe';
COMMENT ON COLUMN downtime_history.end_time IS 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise';

CREATE TABLE comment_history (
  comment_id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  entry_time uint8 NOT NULL,
  author varchar(255) NOT NULL,
  removed_by varchar(255) DEFAULT NULL,
  comment text NOT NULL,
  entry_type comment_type NOT NULL,
  is_persistent bool NOT NULL,
  is_sticky bool NOT NULL,
  expire_time uint8 DEFAULT NULL,
  remove_time uint8 DEFAULT NULL,
  has_been_removed bool NOT NULL,

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
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  start_time uint8 NOT NULL,
  end_time uint8 DEFAULT NULL,
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

COMMENT ON COLUMN flapping_history.id IS 'sha1(environment.name + "host"|"service" + host|service.name + start_time)';
COMMENT ON COLUMN flapping_history.environment_id IS 'environment.id';
COMMENT ON COLUMN flapping_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN flapping_history.host_id IS 'host.id';
COMMENT ON COLUMN flapping_history.service_id IS 'service.id';

CREATE TABLE acknowledgement_history (
  id bin20 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,

  set_time uint8 NOT NULL,
  clear_time uint8 DEFAULT NULL,
  author varchar(255) NOT NULL,
  cleared_by varchar(255) DEFAULT NULL,
  comment text DEFAULT NULL,
  expire_time uint8 DEFAULT NULL,
  is_sticky bool NOT NULL,
  is_persistent bool NOT NULL,

  CONSTRAINT pk_acknowledgement_history PRIMARY KEY (id)
);

ALTER TABLE acknowledgement_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN endpoint_id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE acknowledgement_history ALTER COLUMN service_id SET STORAGE PLAIN;

COMMENT ON COLUMN acknowledgement_history.id IS 'sha1(environment.name + "host"|"service" + host|service.name + set_time)';
COMMENT ON COLUMN acknowledgement_history.environment_id IS 'environment.id';
COMMENT ON COLUMN acknowledgement_history.endpoint_id IS 'endpoint.id';
COMMENT ON COLUMN acknowledgement_history.host_id IS 'host.id';
COMMENT ON COLUMN acknowledgement_history.service_id IS 'service.id';

CREATE TABLE history (
  id bin16 NOT NULL,
  environment_id bin20 NOT NULL,
  endpoint_id bin20 DEFAULT NULL,
  object_type checkable_type NOT NULL,
  host_id bin20 NOT NULL,
  service_id bin20 DEFAULT NULL,
  notification_history_id bin16 DEFAULT NULL,
  state_history_id bin16 DEFAULT NULL,
  downtime_history_id bin20 DEFAULT NULL,
  comment_history_id bin20 DEFAULT NULL,
  flapping_history_id bin20 DEFAULT NULL,
  acknowledgement_history_id bin20 DEFAULT NULL,

  event_type history_type NOT NULL,
  event_time uint8 NOT NULL,

  CONSTRAINT pk_history PRIMARY KEY (id)
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

COMMENT ON COLUMN history.id IS 'notification_history_id, state_history_id or UUID';
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
