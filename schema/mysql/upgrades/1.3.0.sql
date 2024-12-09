ALTER TABLE host MODIFY COLUMN icon_image_alt TEXT NOT NULL;
ALTER TABLE service MODIFY COLUMN icon_image_alt TEXT NOT NULL;

ALTER TABLE endpoint MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE comment MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE notification MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';

ALTER TABLE timeperiod_range MODIFY COLUMN range_value text NOT NULL;

ALTER TABLE checkcommand_argument MODIFY COLUMN argument_key varchar(255) NOT NULL;
ALTER TABLE checkcommand_argument MODIFY COLUMN argument_key_override varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL;
ALTER TABLE eventcommand_argument MODIFY COLUMN argument_key varchar(255) NOT NULL;
ALTER TABLE eventcommand_argument MODIFY COLUMN argument_key_override varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL;
ALTER TABLE notificationcommand_argument MODIFY COLUMN argument_key varchar(255) NOT NULL;
ALTER TABLE notificationcommand_argument MODIFY COLUMN argument_key_override varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL;

ALTER TABLE checkcommand_envvar MODIFY COLUMN envvar_key varchar(255) NOT NULL;
ALTER TABLE eventcommand_envvar MODIFY COLUMN envvar_key varchar(255) NOT NULL;
ALTER TABLE notificationcommand_envvar MODIFY COLUMN envvar_key varchar(255) NOT NULL;

ALTER TABLE host ADD COLUMN affected_children int unsigned DEFAULT NULL AFTER check_retry_interval;
ALTER TABLE host_state ADD COLUMN affects_children enum('n', 'y') NOT NULL DEFAULT 'n' AFTER in_downtime;
ALTER TABLE host_state MODIFY COLUMN affects_children enum('n', 'y') NOT NULL;

ALTER TABLE service ADD COLUMN affected_children int unsigned DEFAULT NULL AFTER check_retry_interval;
ALTER TABLE service_state ADD COLUMN affects_children enum('n', 'y') NOT NULL DEFAULT 'n' AFTER in_downtime;
ALTER TABLE service_state MODIFY COLUMN affects_children enum('n', 'y') NOT NULL;

CREATE TABLE redundancy_group (
  id binary(20) NOT NULL COMMENT 'sha1(name + all-member-parent-names)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  name text NOT NULL,

  CONSTRAINT pk_redundancy_group PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE redundancy_group_state (
  id binary(20) NOT NULL COMMENT 'redundancy_group.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  redundancy_group_id binary(20) NOT NULL COMMENT 'redundancy_group.id',
  failed enum('n', 'y') NOT NULL,
  is_reachable enum('n', 'y') NOT NULL,
  last_state_change BIGINT UNSIGNED NOT NULL,

  CONSTRAINT pk_redundancy_group_state PRIMARY KEY (id),

  UNIQUE INDEX idx_redundancy_group_state_redundancy_group_id (redundancy_group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE dependency (
  id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  name text NOT NULL,
  display_name text NOT NULL,
  name_checksum binary(20) NOT NULL COMMENT 'sha1(name)',
  properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)',
  redundancy_group_id binary(20) DEFAULT NULL COMMENT 'redundancy_group.id',
  timeperiod_id binary(20) DEFAULT NULL COMMENT 'timeperiod.id',
  disable_checks enum('n', 'y') NOT NULL,
  disable_notifications enum('n', 'y') NOT NULL,
  ignore_soft_states enum('n', 'y') NOT NULL,
  states tinyint unsigned NOT NULL,

  CONSTRAINT pk_dependency PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE dependency_state (
  id binary(20) NOT NULL COMMENT 'dependency.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  dependency_id binary(20) NOT NULL COMMENT 'dependency.id',
  failed enum('n', 'y') NOT NULL,

  CONSTRAINT pk_dependency_state PRIMARY KEY (id),

  UNIQUE INDEX idx_dependency_state_dependency_id (dependency_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE dependency_node (
  id binary(20) NOT NULL COMMENT 'host.id|service.id|redundancy_group.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',
  redundancy_group_id binary(20) DEFAULT NULL COMMENT 'redundancy_group.id',

  CONSTRAINT pk_dependency_node PRIMARY KEY (id),

  UNIQUE INDEX idx_dependency_node_host_service_redundancygroup_id (host_id, service_id, redundancy_group_id),
  CONSTRAINT ck_dependency_node_either_checkable_or_redundancy_group_id CHECK (IF(host_id IS NULL, 1, 0) + IF(service_id IS NULL, 1, 0) + IF(redundancy_group_id IS NULL, 1, 0) <= 2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE dependency_edge (
  id binary(20) NOT NULL COMMENT 'sha1(from_node_id + to_node_id + [dependency.id])',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  from_node_id binary(20) NOT NULL COMMENT 'host.id|service.id|redundancy_group.id',
  to_node_id binary(20) NOT NULL COMMENT 'host.id|service.id|redundancy_group.id',
  dependency_id binary(20) DEFAULT NULL COMMENT 'dependency.id',

  CONSTRAINT pk_dependency_edge PRIMARY KEY (id),

  UNIQUE INDEX idx_dependency_edge_from_node_to_node_dependency_id (from_node_id, to_node_id, dependency_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (6, UNIX_TIMESTAMP() * 1000);
