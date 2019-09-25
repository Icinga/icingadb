SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

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