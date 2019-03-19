SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE customvar (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name + value)',
  env_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  reference_counter int(6) unsigned NOT NULL COMMENT 'if 0, the custom var needs to be deleted',

  name varchar(255) NOT NULL COLLATE utf8_bin,
  value text NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=COMPRESSED DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE customvar_flat (
  customvar_id binary(20) NOT NULL COMMENT 'sha1(customvar.id)',
  env_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  path_checksum binary(20) NOT NULL COMMENT 'sha1(flatname before conversion)',
  flatname_checksum binary(20) NOT NULL COMMENT 'sha1(flatname after conversion)',

  flatname varchar(512) NOT NULL COLLATE utf8_bin COMMENT 'Path converted with `.` and `[ ]`',
  flatvalue text NOT NULL,

  PRIMARY KEY (customvar_id, path_checksum)
) ENGINE=InnoDb ROW_FORMAT=COMPRESSED DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;