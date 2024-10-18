CREATE TABLE `dependency` (
  `id` binary(20) NOT NULL,
  `name` text NOT NULL,
  `display_name` text NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE `dependency_state` (
  `id` binary(20) NOT NULL,
  `dependency_id` binary(20) NOT NULL,
  `failed` enum('n', 'y') NOT NULL,
  UNIQUE INDEX `dependency_state_dependency_id_uindex` (dependency_id),
  KEY `dependency_state_dependency_id_fk` (`dependency_id`),
  CONSTRAINT `dependency_state_dependency_id_fk` FOREIGN KEY (`dependency_id`) REFERENCES `dependency` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE `redundancy_group` (
  `id` binary(20) NOT NULL,
  `name` text NOT NULL,
  `display_name` text NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE `redundancy_group_state` (
  `id` binary(20) NOT NULL,
  `redundancy_group_id` binary(20) NOT NULL,
  `failed` enum('n', 'y') NOT NULL,
  `last_state_change` bigint unsigned NOT NULL,
  UNIQUE INDEX `redundancy_group_state_redundancy_group_id_uindex` (redundancy_group_id),
  KEY `redundancy_group_state_redundancy_group_id_fk` (`redundancy_group_id`),
  CONSTRAINT `redundancy_group_state_redundancy_group_id_fk` FOREIGN KEY (`redundancy_group_id`) REFERENCES `redundancy_group` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE `dependency_node` (
  `id` binary(20) NOT NULL,
  `host_id` binary(20) DEFAULT NULL,
  `service_id` binary(20) DEFAULT NULL,
  `redundancy_group_id` binary(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `dependency_node_host_id_service_id_uindex` (`host_id`,`service_id`),
  KEY `dependency_node_redundancy_group_id_fk` (`redundancy_group_id`),
  KEY `dependency_node_service_id_fk` (`service_id`),
  CONSTRAINT `dependency_node_host_id_fk` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`),
  CONSTRAINT `dependency_node_redundancy_group_id_fk` FOREIGN KEY (`redundancy_group_id`) REFERENCES `redundancy_group` (`id`),
  CONSTRAINT `dependency_node_service_id_fk` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE `dependency_edge` (
  `to_node_id` binary(20) NOT NULL,
  `from_node_id` binary(20) NOT NULL,
  `dependency_id` binary(20) DEFAULT NULL,
  UNIQUE KEY `dependency_edge_to_node_id_from_node_id_uindex` (`to_node_id`,`from_node_id`),
  KEY `dependency_edge_dependency_node_id_fk_2` (`from_node_id`),
  KEY `dependency_edge_dependency_id_fk` (`dependency_id`),
  CONSTRAINT `dependency_edge_dependency_id_fk` FOREIGN KEY (`dependency_id`) REFERENCES `dependency` (`id`),
  CONSTRAINT `dependency_edge_dependency_node_id_fk` FOREIGN KEY (`to_node_id`) REFERENCES `dependency_node` (`id`),
  CONSTRAINT `dependency_edge_dependency_node_id_fk_2` FOREIGN KEY (`from_node_id`) REFERENCES `dependency_node` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
