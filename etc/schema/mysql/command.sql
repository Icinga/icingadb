SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE command (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + type + name)',
  zone_id binary(20) DEFAULT NULL COMMENT 'zone.id',
  env_id binary(20) NOT NULL COMMENT 'env.id',

  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  customvars_checksum binary(20) NOT NULL COMMENT 'sha1(command.vars)',
  arguments_checksum binary(20) NOT NULL COMMENT 'sha1(arguments from command_argument where command_argument.command_id = id)',
  envvars_checksum binary(20) NOT NULL COMMENT 'sha1(envvars from command_envvar where command_envvar.command_id = id)',

  type enum('CheckCommand','NotificationCommand','EventCommand') NOT NULL,
  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  command text NOT NULL,
  timeout smallint(5) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE command_argument (
  command_id binary(20) NOT NULL COMMENT 'command.id',
  argument_key varchar(64) NOT NULL,
  env_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  argument_value text DEFAULT NULL,
  argument_order tinyint(3) DEFAULT NULL,
  description text DEFAULT NULL,
  argument_key_override varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  repeat_key enum('y','n') NOT NULL,
  required enum('y','n') NOT NULL,
  set_if varchar(255) DEFAULT NULL,
  skip_key enum('y','n') NOT NULL,

  PRIMARY KEY (command_id,argument_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE command_envvar (
  command_id binary(20) NOT NULL COMMENT 'command.id',
  envvar_key varchar(64) NOT NULL,
  env_id binary(20) NOT NULL COMMENT 'env.id',

  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',

  envvar_value text NOT NULL,

  PRIMARY KEY (command_id,envvar_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE command_customvar (
  command_id binary(20) NOT NULL COMMENT 'command.id',
  customvar_id binary(20) NOT NULL COMMENT 'customvar.id',
  env_id binary(20) DEFAULT NULL COMMENT 'sha1(environment.name)',
  PRIMARY KEY (customvar_id, command_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

/* TODO(el): Default custom variables are missing */
