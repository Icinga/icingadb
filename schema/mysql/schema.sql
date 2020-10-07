-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

SET SESSION sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET SESSION innodb_strict_mode = 1;

CREATE TABLE host (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  address varchar(255) NOT NULL,
  address6 varchar(255) NOT NULL,
  address_bin binary(4) DEFAULT NULL,
  address6_bin binary(16) DEFAULT NULL,

  checkcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'checkcommand.name',
  checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',

  max_check_attempts int unsigned NOT NULL,

  check_timeperiod varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'timeperiod.name',
  check_timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  check_timeout int unsigned DEFAULT NULL,
  check_interval int unsigned NOT NULL,
  check_retry_interval int unsigned NOT NULL,

  active_checks_enabled enum('n', 'y') NOT NULL,
  passive_checks_enabled enum('n', 'y') NOT NULL,
  event_handler_enabled enum('n', 'y') NOT NULL,
  notifications_enabled enum('n', 'y') NOT NULL,

  flapping_enabled enum('n', 'y') NOT NULL,
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  perfdata_enabled enum('n', 'y') NOT NULL,

  eventcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'eventcommand.name',
  eventcommand_id binary(20) DEFAULT NULL COMMENT 'eventcommand.id',

  is_volatile enum('n', 'y') NOT NULL,

  action_url_id binary(20) DEFAULT NULL COMMENT 'action_url.id',
  notes_url_id binary(20) DEFAULT NULL COMMENT 'notes_url.id',
  notes text NOT NULL,
  icon_image_id binary(20) DEFAULT NULL COMMENT 'icon_image.id',
  icon_image_alt varchar(32) NOT NULL,

  zone varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'zone.name',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  command_endpoint varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'endpoint.name',
  command_endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',

  PRIMARY KEY (id),
  KEY idx_action_url_checksum (action_url_id) COMMENT 'cleanup',
  KEY idx_notes_url_checksum (notes_url_id) COMMENT 'cleanup',
  KEY idx_icon_image_checksum (icon_image_id) COMMENT 'cleanup',

  INDEX idx_host_display_name (display_name) COMMENT 'Host list filtered/ordered by display_name',
  INDEX idx_host_name_ci (name_ci) COMMENT 'Host list filtered using quick search',
  INDEX idx_host_name (name) COMMENT 'Host list filtered/ordered by name; Host detail filter'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup_member (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + host_id + hostgroup_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  hostgroup_id binary(20) NOT NULL COMMENT 'hostgroup.id',

  PRIMARY KEY (id),

  INDEX idx_hostgroup_member_host_id (host_id, hostgroup_id),
  INDEX idx_hostgroup_member_hostgroup_id (hostgroup_id, host_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE host_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + host_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_host_customvar_host_id (host_id, customvar_id),
  INDEX idx_host_customvar_customvar_id (customvar_id, host_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + hostgroup_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  hostgroup_id binary(20) NOT NULL COMMENT 'hostgroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_hostgroup_customvar_hostgroup_id (hostgroup_id, customvar_id),
  INDEX idx_hostgroup_customvar_customvar_id (customvar_id, hostgroup_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE host_state (
  id binary(20) NOT NULL COMMENT 'host.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint unsigned NOT NULL,
  hard_state tinyint unsigned NOT NULL,
  previous_hard_state tinyint unsigned NOT NULL,
  attempt tinyint unsigned NOT NULL,
  severity smallint unsigned NOT NULL,

  output longtext DEFAULT NULL,
  long_output longtext DEFAULT NULL,
  performance_data longtext DEFAULT NULL,
  normalized_performance_data longtext DEFAULT NULL,
  check_commandline text DEFAULT NULL,

  is_problem enum('n', 'y') NOT NULL,
  is_handled enum('n', 'y') NOT NULL,
  is_reachable enum('n', 'y') NOT NULL,
  is_flapping enum('n', 'y') NOT NULL,
  is_overdue enum('n', 'y') NOT NULL,

  is_acknowledged enum('n', 'y', 'sticky') NOT NULL,
  acknowledgement_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id',
  last_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id',

  in_downtime enum('n', 'y') NOT NULL,

  execution_time int unsigned DEFAULT NULL,
  latency int unsigned DEFAULT NULL,
  timeout int unsigned DEFAULT NULL,
  check_source text DEFAULT NULL,
  scheduling_source text DEFAULT NULL,

  last_update bigint unsigned DEFAULT NULL,
  last_state_change bigint unsigned NOT NULL,
  next_check bigint unsigned NOT NULL,
  next_update bigint unsigned NOT NULL,

  PRIMARY KEY (id),
  UNIQUE INDEX idx_host_state_host_id (host_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE service (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  host_id binary(20) NOT NULL COMMENT 'sha1(host.id)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  checkcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'checkcommand.name',
  checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',

  max_check_attempts int unsigned NOT NULL,

  check_timeperiod varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'timeperiod.name',
  check_timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  check_timeout int unsigned DEFAULT NULL,
  check_interval int unsigned NOT NULL,
  check_retry_interval int unsigned NOT NULL,

  active_checks_enabled enum('n', 'y') NOT NULL,
  passive_checks_enabled enum('n', 'y') NOT NULL,
  event_handler_enabled enum('n', 'y') NOT NULL,
  notifications_enabled enum('n', 'y') NOT NULL,

  flapping_enabled enum('n', 'y') NOT NULL,
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  perfdata_enabled enum('n', 'y') NOT NULL,

  eventcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'eventcommand.name',
  eventcommand_id binary(20) DEFAULT NULL COMMENT 'eventcommand.id',

  is_volatile enum('n', 'y') NOT NULL,

  action_url_id binary(20) DEFAULT NULL COMMENT 'action_url.id',
  notes_url_id binary(20) DEFAULT NULL COMMENT 'notes_url.id',
  notes text NOT NULL,
  icon_image_id binary(20) DEFAULT NULL COMMENT 'icon_image.id',
  icon_image_alt varchar(32) NOT NULL,

  zone varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'zone.name',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  command_endpoint varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'endpoint.name',
  command_endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',

  PRIMARY KEY (id),

  INDEX idx_service_display_name (display_name) COMMENT 'Service list filtered/ordered by display_name',
  INDEX idx_service_host_id (host_id, display_name) COMMENT 'Service list filtered by host and ordered by display_name',
  INDEX idx_service_name_ci (name_ci) COMMENT 'Service list filtered using quick search',
  INDEX idx_service_name (name) COMMENT 'Service list filtered/ordered by name; Service detail filter'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE servicegroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE servicegroup_member (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + servicegroup_id + service_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  service_id binary(20) NOT NULL COMMENT 'service.id',
  servicegroup_id binary(20) NOT NULL COMMENT 'servicegroup.id',

  PRIMARY KEY (id),

  INDEX idx_servicegroup_member_service_id (service_id, servicegroup_id),
  INDEX idx_servicegroup_member_servicegroup_id (servicegroup_id, service_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE service_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + service_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  service_id binary(20) NOT NULL COMMENT 'service.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),


  INDEX idx_service_customvar_service_id (service_id, customvar_id),
  INDEX idx_service_customvar_customvar_id (customvar_id, service_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE servicegroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + servicegroup_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  servicegroup_id binary(20) NOT NULL COMMENT 'servicegroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_servicegroup_customvar_servicegroup_id (servicegroup_id, customvar_id),
  INDEX idx_servicegroup_customvar_customvar_id (customvar_id, servicegroup_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE service_state (
  id binary(20) NOT NULL COMMENT 'service.id',
  service_id binary(20) NOT NULL COMMENT 'service.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint unsigned NOT NULL,
  hard_state tinyint unsigned NOT NULL,
  previous_hard_state tinyint unsigned NOT NULL,
  attempt tinyint unsigned NOT NULL,
  severity smallint unsigned NOT NULL,

  output longtext DEFAULT NULL,
  long_output longtext DEFAULT NULL,
  performance_data longtext DEFAULT NULL,
  normalized_performance_data longtext DEFAULT NULL,

  check_commandline text DEFAULT NULL,

  is_problem enum('n', 'y') NOT NULL,
  is_handled enum('n', 'y') NOT NULL,
  is_reachable enum('n', 'y') NOT NULL,
  is_flapping enum('n', 'y') NOT NULL,
  is_overdue enum('n', 'y') NOT NULL,

  is_acknowledged enum('n', 'y', 'sticky') NOT NULL,
  acknowledgement_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id',
  last_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id',

  in_downtime enum('n', 'y') NOT NULL,

  execution_time int unsigned DEFAULT NULL,
  latency int unsigned DEFAULT NULL,
  timeout int unsigned DEFAULT NULL,
  check_source text DEFAULT NULL,
  scheduling_source text DEFAULT NULL,

  last_update bigint unsigned DEFAULT NULL,
  last_state_change bigint unsigned NOT NULL,
  next_check bigint unsigned NOT NULL,
  next_update bigint unsigned NOT NULL,

  PRIMARY KEY (id),
  UNIQUE INDEX idx_service_state_service_id (service_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE endpoint (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) NOT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE environment (
  id binary(20) NOT NULL COMMENT 'sha1(name)',
  name varchar(255) NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE icingadb_instance (
  id binary(16) NOT NULL COMMENT 'UUIDv4',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  heartbeat bigint unsigned NOT NULL COMMENT '*nix timestamp',
  responsible enum('n', 'y') NOT NULL,

  icinga2_version varchar(255) NOT NULL,
  icinga2_start_time bigint unsigned NOT NULL,
  icinga2_notifications_enabled enum('n', 'y') NOT NULL,
  icinga2_active_service_checks_enabled enum('n', 'y') NOT NULL,
  icinga2_active_host_checks_enabled enum('n', 'y') NOT NULL,
  icinga2_event_handlers_enabled enum('n', 'y') NOT NULL,
  icinga2_flap_detection_enabled enum('n', 'y') NOT NULL,
  icinga2_performance_data_enabled enum('n', 'y') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE icingadb_schema (
  id int unsigned NOT NULL AUTO_INCREMENT,
  version smallint unsigned NOT NULL,
  timestamp bigint unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (2, CURRENT_TIMESTAMP() * 1000);

CREATE TABLE checkcommand (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout int unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE checkcommand_argument (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + argument_key)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('n', 'y') NOT NULL,
  required enum('n', 'y') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('n', 'y') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE checkcommand_envvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + envvar_key)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE checkcommand_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_checkcommand_customvar_command_id (command_id, customvar_id),
  INDEX idx_checkcommand_customvar_customvar_id (customvar_id, command_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;


CREATE TABLE eventcommand (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout smallint unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE eventcommand_argument (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + argument_key)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('n', 'y') NOT NULL,
  required enum('n', 'y') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('n', 'y') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE eventcommand_envvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + envvar_key)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE eventcommand_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_eventcommand_customvar_command_id (command_id, customvar_id),
  INDEX idx_eventcommand_customvar_customvar_id (customvar_id, command_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout smallint unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand_argument (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + argument_key)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order smallint DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('n', 'y') NOT NULL,
  required enum('n', 'y') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('n', 'y') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand_envvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + envvar_key)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_notificationcommand_customvar_command_id (command_id, customvar_id),
  INDEX idx_notificationcommand_customvar_customvar_id (customvar_id, command_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE comment (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,
  name varchar(548) NOT NULL COMMENT '255+1+255+1+36, i.e. "host.name!service.name!UUID"',

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  text text NOT NULL,
  entry_type enum('comment','ack') NOT NULL,
  entry_time bigint unsigned NOT NULL,
  is_persistent enum('n', 'y') NOT NULL,
  is_sticky enum('n', 'y') NOT NULL,
  expire_time bigint unsigned DEFAULT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id),

  INDEX idx_comment_name (name) COMMENT 'Comment detail filter',
  INDEX idx_comment_entry_time (entry_time) COMMENT 'Comment list fileted/ordered by entry_time'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE downtime (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  triggered_by_id binary(20) DEFAULT NULL COMMENT 'The ID of the downtime that triggered this downtime. This is set when creating downtimes on a host or service higher up in the dependency chain using the "child_option" "DowntimeTriggeredChildren" and can also be set manually via the API.',
  parent_id binary(20) DEFAULT NULL COMMENT 'For service downtimes, the ID of the host downtime that created this downtime by using the "all_services" flag of the schedule-downtime API.',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  name varchar(548) NOT NULL COMMENT '255+1+255+1+36, i.e. "host.name!service.name!UUID"',

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_time bigint unsigned NOT NULL,
  scheduled_start_time bigint unsigned NOT NULL,
  scheduled_end_time bigint unsigned NOT NULL,
  flexible_duration bigint unsigned NOT NULL,
  is_flexible enum('n', 'y') NOT NULL,

  is_in_effect enum('n', 'y') NOT NULL,
  start_time bigint unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  end_time bigint unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + flexible_duration otherwise',
  scheduled_by varchar(767) DEFAULT NULL COMMENT 'Name of the ScheduledDowntime which created this Downtime. 255+1+255+1+255, i.e. "host.name!service.name!scheduled-downtime-name"',

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id),

  INDEX idx_downtime_is_in_effect (is_in_effect, start_time) COMMENT 'Downtime list filtered/ordered by severity',
  INDEX idx_downtime_name (name) COMMENT 'Downtime detail filter'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',

  times_begin int unsigned DEFAULT NULL,
  times_end int unsigned DEFAULT NULL,
  notification_interval int unsigned NOT NULL,
  timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  states tinyint unsigned NOT NULL,
  types smallint unsigned NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id),

  INDEX idx_host_id (host_id),
  INDEX idx_service_id (service_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_user (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + user_id)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  user_id binary(20) NOT NULL COMMENT 'user.id',

  PRIMARY KEY (id),

  INDEX idx_notification_user_user_id (user_id, notification_id),
  INDEX idx_notification_user_notification_id (notification_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_usergroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + usergroup_id)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',

  PRIMARY KEY (id),

  INDEX idx_notification_usergroup_usergroup_id (usergroup_id, notification_id),
  INDEX idx_notification_usergroup_notification_id (notification_id, usergroup_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_recipient (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + (user_id | usergroup_id))',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  user_id binary(20) NULL COMMENT 'user.id',
  usergroup_id binary(20) NULL COMMENT 'usergroup.id',

  PRIMARY KEY (id),

  INDEX idx_notification_recipient_user_id (user_id, notification_id),
  INDEX idx_notification_recipient_notification_id_user (notification_id, user_id),
  INDEX idx_notification_recipient_usergroup_id (usergroup_id, notification_id),
  INDEX idx_notification_recipient_notification_id_usergroup (notification_id, usergroup_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_notification_customvar_notification_id (notification_id, customvar_id),
  INDEX idx_notification_customvar_customvar_id (customvar_id, notification_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE icon_image (
  id binary(20) NOT NULL COMMENT 'sha1(icon_image)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  icon_image text COLLATE utf8mb4_unicode_ci NOT NULL,

  PRIMARY KEY (environment_id, id),
  KEY idx_icon_image (icon_image(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE action_url (
  id binary(20) NOT NULL COMMENT 'sha1(action_url)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  action_url text COLLATE utf8mb4_unicode_ci NOT NULL,

  PRIMARY KEY (environment_id, id),
  KEY idx_action_url (action_url(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notes_url (
  id binary(20) NOT NULL COMMENT 'sha1(notes_url)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  notes_url text COLLATE utf8mb4_unicode_ci NOT NULL,

  PRIMARY KEY (environment_id, id),
  KEY idx_notes_url (notes_url(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod (
  id binary(20) NOT NULL COMMENT 'sha1(env.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  prefer_includes enum('n', 'y') NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_range (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + range_id + timeperiod_id)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  range_key varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  range_value varchar(255) NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_override_include (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + include_id + timeperiod_id)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  override_id binary(20) NOT NULL COMMENT 'timeperiod.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_override_exclude (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + exclude_id + timeperiod_id)',
  environment_id binary(20) NOT NULL COMMENT 'env.id',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  override_id binary(20) NOT NULL COMMENT 'timeperiod.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + timeperiod_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_timeperiod_customvar_timeperiod_id (timeperiod_id, customvar_id),
  INDEX idx_timeperiod_customvar_customvar_id (customvar_id, timeperiod_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name + value)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',

  name varchar(255) NOT NULL,
  value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE customvar_flat (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + flatname + flatvalue)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  customvar_id binary(20) NOT NULL COMMENT 'sha1(customvar.id)',
  flatname_checksum binary(20) NOT NULL COMMENT 'sha1(flatname after conversion)',

  flatname varchar(512) NOT NULL COMMENT 'Path converted with `.` and `[ ]`',
  flatvalue text NOT NULL,

  PRIMARY KEY (id),

  INDEX idx_customvar_flat_customvar_id (customvar_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE user (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  email varchar(255) NOT NULL,
  pager varchar(255) NOT NULL,

  notifications_enabled enum('n', 'y') NOT NULL,

  timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  states tinyint unsigned NOT NULL,
  types smallint unsigned NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id),

  INDEX idx_user_display_name (display_name) COMMENT 'User list filtered/ordered by display_name',
  INDEX idx_user_name_ci (name_ci) COMMENT 'User list filtered using quick search',
  INDEX idx_user_name (name) COMMENT 'User list filtered/ordered by name; User detail filter'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE usergroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id),

  INDEX idx_usergroup_display_name (display_name) COMMENT 'Usergroup list filtered/ordered by display_name',
  INDEX idx_usergroup_name_ci (name_ci) COMMENT 'Usergroup list filtered using quick search',
  INDEX idx_usergroup_name (name) COMMENT 'Usergroup list filtered/ordered by name; Usergroup detail filter'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE usergroup_member (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + usergroup_id + user_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  user_id binary(20) NOT NULL COMMENT 'user.id',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',

  PRIMARY KEY (id),

  INDEX idx_usergroup_member_user_id (user_id, usergroup_id),
  INDEX idx_usergroup_member_usergroup_id (usergroup_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE user_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + user_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  user_id binary(20) NOT NULL COMMENT 'user.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_user_customvar_user_id (user_id, customvar_id),
  INDEX idx_user_customvar_customvar_id (customvar_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE usergroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + usergroup_id + customvar_id)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',

  PRIMARY KEY (id),

  INDEX idx_usergroup_customvar_usergroup_id (usergroup_id, customvar_id),
  INDEX idx_usergroup_customvar_customvar_id (customvar_id, usergroup_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE zone (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  is_global enum('n', 'y') NOT NULL,
  parent_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  depth tinyint unsigned NOT NULL,

  PRIMARY KEY (id),
  INDEX idx_parent_id (parent_id),
  UNIQUE INDEX idx_environment_id_id (environment_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',

  type enum('downtime_start', 'downtime_end', 'downtime_removed', 'custom', 'acknowledgement', 'problem', 'recovery', 'flapping_start', 'flapping_end') NOT NULL,
  send_time bigint unsigned NOT NULL,
  state tinyint unsigned NOT NULL,
  previous_hard_state tinyint unsigned NOT NULL,
  author text NOT NULL,
  `text` text NOT NULL,
  users_notified smallint unsigned NOT NULL,

  PRIMARY KEY (id),

  INDEX idx_notification_history_event_time (send_time DESC) COMMENT 'Notification list filtered/ordered by entry_time'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE user_notification_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  notification_history_id binary(16) NOT NULL COMMENT 'UUID notification_history.id',
  user_id binary(20) NOT NULL COMMENT 'user.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE state_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  event_time bigint unsigned NOT NULL,
  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint unsigned NOT NULL,
  hard_state tinyint unsigned NOT NULL,
  previous_soft_state tinyint unsigned NOT NULL,
  previous_hard_state tinyint unsigned NOT NULL,
  attempt tinyint unsigned NOT NULL,
  output longtext DEFAULT NULL,
  long_output longtext DEFAULT NULL,
  max_check_attempts int unsigned NOT NULL,
  check_source text DEFAULT NULL,
  scheduling_source text DEFAULT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE downtime_history (
  downtime_id binary(20) NOT NULL COMMENT 'downtime.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  triggered_by_id binary(20) DEFAULT NULL COMMENT 'The ID of the downtime that triggered this downtime. This is set when creating downtimes on a host or service higher up in the dependency chain using the "child_option" "DowntimeTriggeredChildren" and can also be set manually via the API.',
  parent_id binary(20) DEFAULT NULL COMMENT 'For service downtimes, the ID of the host downtime that created this downtime by using the "all_services" flag of the schedule-downtime API.',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  entry_time bigint unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  cancelled_by varchar(255) DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  is_flexible enum('n', 'y') NOT NULL,
  flexible_duration bigint unsigned NOT NULL,
  scheduled_start_time bigint unsigned NOT NULL,
  scheduled_end_time bigint unsigned NOT NULL,
  start_time bigint unsigned NOT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  end_time bigint unsigned NOT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise',
  scheduled_by varchar(767) DEFAULT NULL COMMENT 'Name of the ScheduledDowntime which created this Downtime. 255+1+255+1+255, i.e. "host.name!service.name!scheduled-downtime-name"',
  has_been_cancelled enum('n', 'y') NOT NULL,
  trigger_time bigint unsigned NOT NULL,
  cancel_time bigint unsigned DEFAULT NULL,

  PRIMARY KEY (downtime_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE comment_history (
  comment_id binary(20) NOT NULL COMMENT 'comment.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  entry_time bigint unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  removed_by varchar(255) DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_type enum('comment','ack') NOT NULL,
  is_persistent enum('n', 'y') NOT NULL,
  is_sticky enum('n', 'y') NOT NULL,
  expire_time bigint unsigned DEFAULT NULL,
  remove_time bigint unsigned DEFAULT NULL,
  has_been_removed enum('n', 'y') NOT NULL,

  PRIMARY KEY (comment_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE flapping_history (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + "Host"|"Service" + host|service.name + start_time)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  start_time bigint unsigned NOT NULL,
  end_time bigint unsigned DEFAULT NULL,
  percent_state_change_start float DEFAULT NULL,
  percent_state_change_end float DEFAULT NULL,
  flapping_threshold_low float NOT NULL,
  flapping_threshold_high float NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE acknowledgement_history (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + "Host"|"Service" + host|service.name + set_time)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  set_time bigint unsigned NOT NULL,
  clear_time bigint unsigned DEFAULT NULL,
  author varchar(255) DEFAULT NULL COLLATE utf8mb4_unicode_ci COMMENT 'NULL if ack_set event happened before Icinga DB history recording',
  cleared_by varchar(255) DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  comment text DEFAULT NULL COMMENT 'NULL if ack_set event happened before Icinga DB history recording',
  expire_time bigint unsigned DEFAULT NULL,
  is_sticky enum('n', 'y') DEFAULT NULL COMMENT 'NULL if ack_set event happened before Icinga DB history recording',
  is_persistent enum('n', 'y') DEFAULT NULL COMMENT 'NULL if ack_set event happened before Icinga DB history recording',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE history (
  id binary(16) NOT NULL COMMENT 'notification_history_id, state_history_id or UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',
  notification_history_id binary(16) DEFAULT NULL COMMENT 'notification_history.id',
  state_history_id binary(16) DEFAULT NULL COMMENT 'state_history.id',
  downtime_history_id binary(20) DEFAULT NULL COMMENT 'downtime_history.downtime_id',
  comment_history_id binary(20) DEFAULT NULL COMMENT 'comment_history.comment_id',
  flapping_history_id binary(20) DEFAULT NULL COMMENT 'flapping_history.id',
  acknowledgement_history_id binary(20) DEFAULT NULL COMMENT 'acknowledgement_history.id',

  event_type enum('notification','state_change','downtime_start', 'downtime_end','comment_add','comment_remove','flapping_start','flapping_end','ack_set','ack_clear') NOT NULL,
  event_time bigint unsigned NOT NULL,

  PRIMARY KEY (id),

  INDEX idx_history_event_time (event_time) COMMENT 'History filtered/ordered by event_time',
  INDEX idx_history_acknowledgement (acknowledgement_history_id),
  INDEX idx_history_comment (comment_history_id),
  INDEX idx_history_downtime (downtime_history_id),
  INDEX idx_history_flapping (flapping_history_id),
  INDEX idx_history_notification (notification_history_id),
  INDEX idx_history_state (state_history_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
