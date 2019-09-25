SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

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
  environment_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;
