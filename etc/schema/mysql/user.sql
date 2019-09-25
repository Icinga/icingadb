SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

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
  environment_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE usergroup_customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + usergroup_id + customvar_id)',
  usergroup_id binary(20) NOT NULL COMMENT 'usergroup.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  environment_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;