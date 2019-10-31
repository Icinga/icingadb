SET sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION';
SET innodb_strict_mode = 1;

CREATE TABLE notification_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',
  notification_id binary(20) NOT NULL COMMENT 'notification.id',

  type smallint(3) unsigned NOT NULL,
  event_time bigint(20) unsigned NOT NULL,
  state tinyint(1) unsigned NOT NULL,
  previous_hard_state tinyint(1) unsigned NOT NULL,
  author text DEFAULT NULL,
  `text` text DEFAULT NULL,
  users_notified smallint(5) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE state_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  event_time bigint(20) unsigned NOT NULL,
  state_type enum('hard', 'soft') NOT NULL,
  soft_state tinyint(1) unsigned NOT NULL,
  hard_state tinyint(1) unsigned NOT NULL,
  previous_hard_state tinyint(1) unsigned NOT NULL,
  attempt tinyint(1) unsigned NOT NULL,
  last_soft_state tinyint(1) unsigned NOT NULL,
  last_hard_state tinyint(1) unsigned NOT NULL,
  output text DEFAULT NULL,
  long_output text DEFAULT NULL,
  max_check_attempts int(10) unsigned NOT NULL,
  check_source text DEFAULT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE downtime_history (
  downtime_id binary(20) NOT NULL COMMENT 'downtime.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',
  triggered_by_id binary(20) NULL DEFAULT NULL COMMENT 'downtime.id',

  entry_time bigint(20) unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  is_flexible enum('y', 'n') NOT NULL,
  flexible_duration bigint(20) unsigned NOT NULL,
  scheduled_start_time bigint(20) unsigned NOT NULL,
  scheduled_end_time bigint(20) unsigned NOT NULL,
  was_started enum('y', 'n') NOT NULL,
  actual_start_time bigint(20) unsigned DEFAULT NULL COMMENT 'Time when the host went into a problem state during the downtimes timeframe',
  actual_end_time bigint(20) unsigned DEFAULT NULL COMMENT 'Problem state assumed: scheduled_end_time if fixed, start_time + duration otherwise',
  was_cancelled enum('y', 'n') NOT NULL,
  is_in_effect enum('y', 'n') NOT NULL,
  trigger_time bigint(20) unsigned NOT NULL,
  deletion_time bigint(20) unsigned NULL DEFAULT NULL,

  PRIMARY KEY (downtime_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE comment_history (
  comment_id binary(20) NOT NULL COMMENT 'comment.id',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  entry_time bigint(20) unsigned NOT NULL,
  author varchar(255) NOT NULL COLLATE utf8mb4_unicode_ci,
  comment text NOT NULL,
  entry_type enum('comment','ack','downtime','flapping') NOT NULL,
  is_persistent enum('y','n') NOT NULL,
  expire_time bigint(20) unsigned DEFAULT NULL,
  deletion_time bigint(20) unsigned NULL DEFAULT NULL,

  PRIMARY KEY (comment_id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE flapping_history (
  id binary(16) NOT NULL COMMENT 'UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',

  event_time bigint(20) unsigned NOT NULL,
  event_type enum('flapping_start', 'flapping_end') NOT NULL,
  percent_state_change float unsigned NOT NULL,
  flapping_threshold_low float unsigned NOT NULL,
  flapping_threshold_high float unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;

CREATE TABLE history (
  id binary(16) NOT NULL COMMENT 'notification_history_id, state_history_id, flapping_history_id or UUID',
  environment_id binary(20) NOT NULL COMMENT 'environment.id',
  endpoint_id binary(20) NULL DEFAULT NULL COMMENT 'endpoint.id',
  object_type enum('host', 'service') NOT NULL,
  host_id binary(20) NULL DEFAULT NULL COMMENT 'host.id',
  service_id binary(20) NULL DEFAULT NULL COMMENT 'service.id',
  notification_history_id binary(16) NOT NULL COMMENT 'notification_history.id or 00000000-0000-0000-0000-000000000000',
  state_history_id binary(16) NOT NULL COMMENT 'state_history.id or 00000000-0000-0000-0000-000000000000',
  downtime_history_id binary(20) NOT NULL COMMENT 'downtime_history.downtime_id or 0000000000000000000000000000000000000000',
  comment_history_id binary(20) NOT NULL COMMENT 'comment_history.comment_id or 0000000000000000000000000000000000000000',
  flapping_history_id binary(16) NOT NULL COMMENT 'flapping_history.id or 00000000-0000-0000-0000-000000000000',

  event_type enum('notification','state','downtime_schedule','downtime_start', 'downtime_end','comment_add','comment_remove','flapping_start','flapping_end') NOT NULL,
  event_time bigint(20) unsigned NOT NULL,

  PRIMARY KEY (id)
) ENGINE=InnoDb ROW_FORMAT=DYNAMIC DEFAULT CHARSET=utf8mb4 COLLATE utf8mb4_bin;
