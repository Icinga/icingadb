ALTER TABLE host ADD COLUMN total_children int unsigned DEFAULT NULL AFTER check_retry_interval;
ALTER TABLE host_state ADD COLUMN affects_children enum('n', 'y') NOT NULL DEFAULT 'n' AFTER in_downtime;
ALTER TABLE host_state MODIFY COLUMN affects_children enum('n', 'y') NOT NULL;

ALTER TABLE service ADD COLUMN total_children int unsigned DEFAULT NULL AFTER check_retry_interval;
ALTER TABLE service_state ADD COLUMN affects_children enum('n', 'y') NOT NULL DEFAULT 'n' AFTER in_downtime;
ALTER TABLE service_state MODIFY COLUMN affects_children enum('n', 'y') NOT NULL;

CREATE TABLE redundancy_group (
  id binary(20) NOT NULL COMMENT 'sha1(name + all(member parent_name + timeperiod.name + states + ignore_soft_states))',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  display_name text NOT NULL,

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

CREATE TABLE dependency_node (
  id binary(20) NOT NULL COMMENT 'host.id|service.id|redundancy_group.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  host_id binary(20) DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) DEFAULT NULL COMMENT 'service.id',
  redundancy_group_id binary(20) DEFAULT NULL COMMENT 'redundancy_group.id',

  CONSTRAINT pk_dependency_node PRIMARY KEY (id),

  UNIQUE INDEX idx_dependency_node_host_service_redundancygroup_id (host_id, service_id, redundancy_group_id),
  CONSTRAINT ck_dependency_node_either_checkable_or_redundancy_group_id CHECK (
    IF(redundancy_group_id IS NULL, host_id IS NOT NULL, host_id IS NULL AND service_id IS NULL) = 1
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE dependency_edge_state (
  id binary(20) NOT NULL COMMENT 'sha1([dependency_edge.from_node_id|parent_name + timeperiod.name + states + ignore_soft_states] + dependency_edge.to_node_id)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  failed enum('n', 'y') NOT NULL,

  CONSTRAINT pk_dependency_edge_state PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE dependency_edge (
  id binary(20) NOT NULL COMMENT 'sha1(from_node_id + to_node_id)',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  from_node_id binary(20) NOT NULL COMMENT 'dependency_node.id',
  to_node_id binary(20) NOT NULL COMMENT 'dependency_node.id',
  dependency_edge_state_id binary(20) NOT NULL COMMENT 'dependency_edge_state.id',
  display_name text NOT NULL,

  CONSTRAINT pk_dependency_edge PRIMARY KEY (id),

  UNIQUE INDEX idx_dependency_edge_from_node_to_node_id (from_node_id, to_node_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

ALTER TABLE icingadb_instance ADD COLUMN icingadb_version varchar(255) NOT NULL DEFAULT 'unknown' AFTER icinga2_performance_data_enabled;
ALTER TABLE icingadb_instance MODIFY COLUMN icingadb_version varchar(255) NOT NULL;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (7, UNIX_TIMESTAMP() * 1000);
