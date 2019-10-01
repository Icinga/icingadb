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
  environment_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE host_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + host_id + customvar_id)',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE hostgroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + hostgroup_id + customvar_id)',
  hostgroup_id binary(20) NOT NULL COMMENT 'hostgroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_state (
  host_id binary(20) NOT NULL COMMENT 'host.id',
  environment_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',

  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint(1) unsigned NOT NULL,
  hard_state tinyint(1) unsigned NOT NULL,
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

  execution_time bigint(20) unsigned DEFAULT NULL,
  latency bigint(20) unsigned DEFAULT NULL,
  timeout bigint(20) unsigned DEFAULT NULL,

  last_update bigint(20) unsigned NOT NULL,
  last_state_change bigint(20) unsigned NOT NULL,
  last_soft_state tinyint(1) unsigned NOT NULL,
  last_hard_state tinyint(1) unsigned NOT NULL,
  next_check bigint(20) unsigned NOT NULL,
  next_update bigint(20) unsigned NOT NULL,

  PRIMARY KEY (host_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_notification_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',

  type smallint(3) unsigned NOT NULL,
  send_time bigint(20) unsigned NOT NULL,
  state tinyint(1) unsigned NOT NULL,
  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  users_notified smallint(5) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_state_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',

  change_time bigint(20) unsigned NOT NULL,
  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint(1) unsigned NOT NULL,
  hard_state tinyint(1) unsigned NOT NULL,
  attempt tinyint(1) unsigned NOT NULL,
  last_soft_state tinyint(1) unsigned NOT NULL,
  last_hard_state tinyint(1) unsigned NOT NULL,
  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  max_check_attempts int(10) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_downtime_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  downtime_id binary(20) NOT NULL COMMENT 'host_downtime.id',
  triggered_by_id binary(20) NOT NULL COMMENT 'host_downtime.id',

  entry_time bigint(20) unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  is_fixed enum('y', 'n') NOT NULL,
  duration bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  was_started enum('y', 'n') NOT NULL,
  actual_start_time bigint(20) unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  actual_end_time bigint(20) unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise',
  was_cancelled enum('y', 'n') NOT NULL,
  is_in_effect enum('y', 'n') NOT NULL,
  trigger_time bigint(20) unsigned NOT NULL,
  deletion_time bigint(20) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_comment_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',
  comment_id binary(20) NOT NULL COMMENT 'host_comment.id',

  entry_time bigint(20) unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_type enum('comment','ack','downtime','flapping') NOT NULL,
  is_persistent enum('y','n') NOT NULL,
  expire_time bigint(20) unsigned DEFAULT NULL,
  deletion_time bigint(20) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE host_flapping_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) NOT NULL COMMENT 'host.id',

  event_time bigint(20) unsigned NOT NULL,
  event_type enum('start', 'end') NOT NULL,
  percent_state_change float unsigned NOT NULL,
  flapping_threshold_low float unsigned NOT NULL,
  flapping_threshold_high float unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;
