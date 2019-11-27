-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE host (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(host.vars)',
  groups_checksum binary(20) NOT NULL COMMENT 'sha1(hostgroup.name + hostgroup.name ...)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  address varchar(255) NOT NULL,
  address6 varchar(255) NOT NULL,
  address_bin binary(4) DEFAULT NULL,
  address6_bin binary(16) DEFAULT NULL,

  checkcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'checkcommand.name',
  checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',

  max_check_attempts int(10) unsigned NOT NULL,

  check_timeperiod varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'timeperiod.name',
  check_timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  check_timeout int(10) unsigned DEFAULT NULL,
  check_interval int(10) unsigned NOT NULL,
  check_retry_interval int(10) unsigned NOT NULL,

  active_checks_enabled enum('y','n') NOT NULL,
  passive_checks_enabled enum('y','n') NOT NULL,
  event_handler_enabled enum('y','n') NOT NULL,
  notifications_enabled enum('y','n') NOT NULL,

  flapping_enabled enum('y','n') NOT NULL,
  flapping_threshold_low float unsigned NOT NULL,
  flapping_threshold_high float unsigned NOT NULL,

  perfdata_enabled enum('y','n') NOT NULL,

  eventcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'eventcommand.name',
  eventcommand_id binary(20) DEFAULT NULL COMMENT 'eventcommand.id',

  is_volatile enum('y','n') NOT NULL,

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
  KEY idx_icon_image_checksum (icon_image_id) COMMENT 'cleanup'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(hostgroup.vars)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup_member (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + host_id + hostgroup_id)',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  hostgroup_id binary(20) NOT NULL COMMENT 'hostgroup.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE host_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + host_id + customvar_id)',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + hostgroup_id + customvar_id)',
  hostgroup_id binary(20) NOT NULL COMMENT 'hostgroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_state (
  host_id binary(20) NOT NULL COMMENT 'host.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint(1) unsigned NOT NULL,
  hard_state tinyint(1) unsigned NOT NULL,
  previous_hard_state tinyint(1) unsigned NOT NULL,
  attempt tinyint(1) unsigned NOT NULL,
  severity smallint unsigned NOT NULL,

  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  performance_data text DEFAULT NULL,
  check_commandline text DEFAULT NULL,

  is_problem enum('y', 'n') NOT NULL,
  is_handled enum('y', 'n') NOT NULL,
  is_reachable enum('y', 'n') NOT NULL,
  is_flapping enum('y', 'n') NOT NULL,

  is_acknowledged enum('y', 'n', 'sticky') NOT NULL,
  acknowledgement_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id',

  in_downtime enum('y', 'n') NOT NULL,

  execution_time int(10) unsigned DEFAULT NULL,
  latency int(10) unsigned DEFAULT NULL,
  timeout int(10) unsigned DEFAULT NULL,
  check_source text DEFAULT NULL,

  last_update bigint(20) unsigned NOT NULL,
  last_state_change bigint(20) unsigned NOT NULL,
  next_check bigint(20) unsigned NOT NULL,
  next_update bigint(20) unsigned NOT NULL,

  PRIMARY KEY (host_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE service (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(service.vars)',
  groups_checksum binary(20) NOT NULL COMMENT 'sha1(servicegroup.name + servicegroup.name ...)',
  host_id binary(20) NOT NULL COMMENT 'sha1(host.id)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  checkcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'checkcommand.name',
  checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',

  max_check_attempts int(10) unsigned NOT NULL,

  check_timeperiod varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'timeperiod.name',
  check_timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  check_timeout int(10) unsigned DEFAULT NULL,
  check_interval int(10) unsigned NOT NULL,
  check_retry_interval int(10) unsigned NOT NULL,

  active_checks_enabled enum('y','n') NOT NULL,
  passive_checks_enabled enum('y','n') NOT NULL,
  event_handler_enabled enum('y','n') NOT NULL,
  notifications_enabled enum('y','n') NOT NULL,

  flapping_enabled enum('y','n') NOT NULL,
  flapping_threshold_low float unsigned NOT NULL,
  flapping_threshold_high float unsigned NOT NULL,

  perfdata_enabled enum('y','n') NOT NULL,

  eventcommand varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'eventcommand.name',
  eventcommand_id binary(20) DEFAULT NULL COMMENT 'eventcommand.id',

  is_volatile enum('y','n') NOT NULL,

  action_url_id binary(20) DEFAULT NULL COMMENT 'action_url.id',
  notes_url_id binary(20) DEFAULT NULL COMMENT 'notes_url.id',
  notes text NOT NULL,
  icon_image_id binary(20) DEFAULT NULL COMMENT 'icon_image.id',
  icon_image_alt varchar(32) NOT NULL,

  zone varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'zone.name',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  command_endpoint varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'endpoint.name',
  command_endpoint_id binary(20) DEFAULT NULL COMMENT 'endpoint.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE servicegroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(servicegroup.vars)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE servicegroup_member (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + servicegroup_id + service_id)',
  service_id binary(20) NOT NULL COMMENT 'service.id',
  servicegroup_id binary(20) NOT NULL COMMENT 'servicegroup.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE service_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + service_id + customvar_id)',
  service_id binary(20) NOT NULL COMMENT 'service.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE servicegroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + servicegroup_id + customvar_id)',
  servicegroup_id binary(20) NOT NULL COMMENT 'servicegroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE service_state (
  service_id binary(20) NOT NULL COMMENT 'service.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint(1) unsigned NOT NULL,
  hard_state tinyint(1) unsigned NOT NULL,
  previous_hard_state tinyint(1) unsigned NOT NULL,
  attempt tinyint(1) unsigned NOT NULL,
  severity smallint unsigned NOT NULL,

  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  performance_data text DEFAULT NULL,
  check_commandline text DEFAULT NULL,

  is_problem enum('y', 'n') NOT NULL,
  is_handled enum('y', 'n') NOT NULL,
  is_reachable enum('y', 'n') NOT NULL,
  is_flapping enum('y', 'n') NOT NULL,

  is_acknowledged enum('y', 'n', 'sticky') NOT NULL,
  acknowledgement_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id',

  in_downtime enum('y', 'n') NOT NULL,

  execution_time int(10) unsigned DEFAULT NULL,
  latency int(10) unsigned DEFAULT NULL,
  timeout int(10) unsigned DEFAULT NULL,
  check_source text DEFAULT NULL,

  last_update bigint(20) unsigned NOT NULL,
  last_state_change bigint(20) unsigned NOT NULL,
  next_check bigint(20) unsigned NOT NULL,
  next_update bigint(20) unsigned NOT NULL,

  PRIMARY KEY (service_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

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
  heartbeat bigint(20) unsigned NOT NULL COMMENT '*nix timestamp',
  responsible enum('y','n') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
CREATE TABLE checkcommand (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout int(10) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE checkcommand_argument (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + argument_key)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order tinyint(3) DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('y','n') NOT NULL,
  required enum('y','n') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('y','n') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE checkcommand_envvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + envvar_key)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE checkcommand_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + customvar_id)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;


CREATE TABLE eventcommand (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout smallint(5) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE eventcommand_argument (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + argument_key)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order tinyint(3) DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('y','n') NOT NULL,
  required enum('y','n') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('y','n') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE eventcommand_envvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + envvar_key)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE eventcommand_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + customvar_id)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE notificationcommand (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout smallint(5) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand_argument (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + argument_key)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order tinyint(3) DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('y','n') NOT NULL,
  required enum('y','n') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('y','n') NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand_envvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + envvar_key)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notificationcommand_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + command_id + customvar_id)',
  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE comment (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  text text NOT NULL,
  entry_type enum('comment','ack','downtime','flapping') NOT NULL,
  entry_time bigint(20) unsigned NOT NULL,
  is_persistent enum('y','n') NOT NULL,
  is_sticky enum('y','n') NOT NULL,
  expire_time bigint(20) unsigned DEFAULT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE downtime (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  triggered_by_id binary(20) NULL DEFAULT NULL COMMENT 'downtime.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  name varchar(255) NOT NULL,

  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_time bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  flexible_duration bigint(20) unsigned NOT NULL,
  is_flexible enum('y', 'n') NOT NULL,

  is_in_effect enum('y', 'n') NOT NULL,
  start_time bigint(20) unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  end_time bigint(20) unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + flexible_duration otherwise',

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL,
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(notification.vars)',
  users_checksum binary(20) NOT NULL,
  usergroups_checksum binary(20) NOT NULL,

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  host_id binary(20) NOT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',
  command_id binary(20) NOT NULL COMMENT 'command.id',

  times_begin int(10) unsigned DEFAULT NULL,
  times_end int(10) unsigned DEFAULT NULL,
  notification_interval int(10) unsigned NOT NULL,
  timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  states tinyint(2) unsigned NOT NULL,
  types smallint(3) unsigned NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_user (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + user_id)',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  user_id binary(20) NOT NULL COMMENT 'user.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_usergroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + usergroup_id)',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification_id + customvar_id)',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE icon_image (
  id binary(20) NOT NULL COMMENT 'sha1(icon_image)',
  icon_image text COLLATE utf8mb4_unicode_ci NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (environment_id, id),
  KEY idx_icon_image (icon_image(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE action_url (
  id binary(20) NOT NULL COMMENT 'sha1(action_url)',
  action_url text COLLATE utf8mb4_unicode_ci NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (environment_id, id),
  KEY idx_action_url (action_url(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notes_url (
  id binary(20) NOT NULL COMMENT 'sha1(notes_url)',
  notes_url text COLLATE utf8mb4_unicode_ci NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

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
  prefer_includes enum('y','n') NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_range (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + range_id + timeperiod_id)',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  range_key varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  range_value varchar(255) NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_override_include (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + include_id + timeperiod_id)',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  override_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_override_exclude (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + exclude_id + timeperiod_id)',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  override_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  environment_id binary(20) NOT NULL COMMENT 'env.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE timeperiod_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + timeperiod_id + customvar_id)',
  timeperiod_id binary(20) NOT NULL COMMENT 'timeperiod.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name + value)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',

  name varchar(255) NOT NULL COLLATE utf8_bin,
  value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=COMPRESSED DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE customvar_flat (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + flatname + flatvalue)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  customvar_id binary(20) NOT NULL COMMENT 'sha1(customvar.id)',
  flatname_checksum binary(20) NOT NULL COMMENT 'sha1(flatname after conversion)',

  flatname varchar(512) NOT NULL COLLATE utf8_bin COMMENT 'Path converted with `.` and `[ ]`',
  flatvalue text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=COMPRESSED DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE user (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(user.vars)',
  groups_checksum binary(20) NOT NULL COMMENT 'sha1(usergroup.name + userroup.name ...)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  email varchar(255) NOT NULL,
  pager varchar(255) NOT NULL,

  notifications_enabled enum('y', 'n') NOT NULL,

  timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',

  states tinyint(2) unsigned NOT NULL,
  types smallint(3) unsigned NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE usergroup (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(usergroup.vars)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  display_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE usergroup_member (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + usergroup_id + user_id)',
  user_id binary(20) NOT NULL COMMENT 'user.id',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE user_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + user_id + customvar_id)',
  user_id binary(20) NOT NULL COMMENT 'user.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE usergroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + usergroup_id + customvar_id)',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE zone (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  parents_checksum binary(20) NOT NULL COMMENT 'sha1(all parents checksums)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  is_global enum('y','n') NOT NULL,
  parent_id binary(20) DEFAULT NULL COMMENT 'zone.id',

  depth tinyint(3) unsigned NOT NULL,

  PRIMARY KEY (id),
  INDEX idx_parent_id (parent_id),
  UNIQUE INDEX idx_environment_id_id (environment_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE notification_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',

  type enum('downtime_start', 'downtime_end', 'downtime_removed', 'custom', 'acknowledgement', 'problem', 'recovery', 'flapping_start', 'flapping_end') NOT NULL,
  send_time bigint(20) unsigned NOT NULL,
  state tinyint(1) unsigned NOT NULL,
  previous_hard_state tinyint(1) unsigned NOT NULL,
  author text NOT NULL,
  `text` text NOT NULL,
  users_notified smallint(5) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE user_notification_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  notification_history_id binary(16) NOT NULL COMMENT 'UUID notification_history.id',
  user_id binary(20) NOT NULL COMMENT 'user.id',

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE state_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  event_time bigint(20) unsigned NOT NULL,
  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint(1) unsigned NOT NULL,
  hard_state tinyint(1) unsigned NOT NULL,
  previous_soft_state tinyint(1) unsigned NOT NULL,
  previous_hard_state tinyint(1) unsigned NOT NULL,
  attempt tinyint(1) unsigned NOT NULL,
  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  max_check_attempts int(10) unsigned NOT NULL,
  check_source text DEFAULT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE downtime_history (
  downtime_id binary(20) NOT NULL COMMENT 'downtime.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  triggered_by_id binary(20) NULL DEFAULT NULL COMMENT 'downtime.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  entry_time bigint(20) unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  is_flexible enum('y', 'n') NOT NULL,
  flexible_duration bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  start_time bigint(20) unsigned NOT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  end_time bigint(20) unsigned NOT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise',
  has_been_cancelled enum('y', 'n') NOT NULL,
  trigger_time bigint(20) unsigned NOT NULL,
  cancel_time bigint(20) unsigned NULL DEFAULT NULL,

  PRIMARY KEY (downtime_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE comment_history (
  comment_id binary(20) NOT NULL COMMENT 'comment.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  entry_time bigint(20) unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_type enum('comment','ack','downtime','flapping') NOT NULL,
  is_persistent enum('y','n') NOT NULL,
  is_sticky enum('y','n') NOT NULL,
  expire_time bigint(20) unsigned DEFAULT NULL,
  remove_time bigint(20) unsigned NULL DEFAULT NULL,
  has_been_removed enum('y','n') NOT NULL,

  PRIMARY KEY (comment_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE flapping_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  event_time bigint(20) unsigned NOT NULL,
  event_type enum('flapping_start', 'flapping_end') NOT NULL,
  percent_state_change float unsigned NOT NULL,
  flapping_threshold_low float unsigned NOT NULL,
  flapping_threshold_high float unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE history (
  id binary(16) NOT NULL COMMENT 'notification_history_id, state_history_id, flapping_history_id or UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',
  notification_history_id binary(16) NOT NULL COMMENT 'notification_history.id or 00000000-0000-0000-0000-000000000000',
  state_history_id binary(16) NOT NULL COMMENT 'state_history.id or 00000000-0000-0000-0000-000000000000',
  downtime_history_id binary(20) NOT NULL COMMENT 'downtime_history.downtime_id or 0000000000000000000000000000000000000000',
  comment_history_id binary(20) NOT NULL COMMENT 'comment_history.comment_id or 0000000000000000000000000000000000000000',
  flapping_history_id binary(16) NOT NULL COMMENT 'flapping_history.id or 00000000-0000-0000-0000-000000000000',

  event_type enum('notification','state_change','downtime_schedule','downtime_start', 'downtime_end','comment_add','comment_remove','flapping_start','flapping_end') NOT NULL,
  event_time bigint(20) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;