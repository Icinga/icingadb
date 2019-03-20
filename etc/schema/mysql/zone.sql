SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE zone (
  id binary(20) NOT NULL COMMENT 'sha1(environment.name + name)',
  env_id binary(20) NOT NULL COMMENT 'sha1(environment.name)',
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  parents_checksum binary(20) NOT NULL COMMENT 'sha1(all parents checksums)',

  name varchar(255) NOT NULL,
  name_ci varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,

  is_global enum('y','n') NOT NULL,
  parent_id binary(20) DEFAULT NULL COMMENT 'zone.id',


  PRIMARY KEY (id),
  INDEX idx_parent_id (parent_id),
  UNIQUE INDEX idx_env_id_id (env_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE zone_tree (
  parent_id binary(20) NOT NULL COMMENT 'zone.id',
  child_id binary(20) NOT NULL COMMENT 'zone.id',
  env_id binary(20) NOT NULL COMMENT 'env.id',
  depth tinyint(3) unsigned NOT NULL,

  PRIMARY KEY (child_id,depth),
  UNIQUE INDEX zone_relation (child_id,parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;