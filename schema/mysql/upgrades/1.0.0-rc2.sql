ALTER TABLE host_state DROP PRIMARY KEY;
ALTER TABLE host_state ADD COLUMN id binary(20) NOT NULL COMMENT 'host.id' FIRST;
UPDATE host_state SET id = host_id;
ALTER TABLE host_state ADD PRIMARY KEY (id);
ALTER TABLE host_state ADD COLUMN properties_checksum binary(20) AFTER environment_id;
UPDATE host_state SET properties_checksum = 0;
ALTER TABLE host_state MODIFY COLUMN properties_checksum binary(20) COMMENT 'sha1(all properties)' NOT NULL;
ALTER TABLE host_state ADD UNIQUE INDEX idx_host_state_host_id (host_id);

ALTER TABLE service_state DROP PRIMARY KEY;
ALTER TABLE service_state ADD COLUMN id binary(20) NOT NULL COMMENT 'service.id' FIRST;
UPDATE service_state SET id = service_id;
ALTER TABLE service_state ADD PRIMARY KEY (id);
ALTER TABLE service_state ADD COLUMN properties_checksum binary(20) AFTER environment_id;
UPDATE service_state SET properties_checksum = 0;
ALTER TABLE service_state MODIFY COLUMN properties_checksum binary(20) COMMENT 'sha1(all properties)' NOT NULL;
ALTER TABLE service_state ADD UNIQUE INDEX idx_service_state_service_id (service_id);

ALTER TABLE checkcommand_argument MODIFY COLUMN argument_order smallint DEFAULT NULL;
ALTER TABLE eventcommand_argument MODIFY COLUMN argument_order smallint DEFAULT NULL;
ALTER TABLE notificationcommand_argument MODIFY COLUMN argument_order smallint DEFAULT NULL;

ALTER TABLE acknowledgement_history MODIFY COLUMN id binary(20) NOT NULL COMMENT 'sha1(environment.name + "Host"|"Service" + host|service.name + set_time)';
ALTER TABLE flapping_history MODIFY COLUMN id binary(20) NOT NULL COMMENT 'sha1(environment.name + "Host"|"Service" + host|service.name + start_time)';

ALTER TABLE host ADD INDEX idx_host_name_ci (name_ci) COMMENT 'Host list filtered using quick search';
ALTER TABLE service ADD INDEX idx_service_name_ci (name_ci) COMMENT 'Service list filtered using quick search';

ALTER TABLE user ADD INDEX idx_user_name_ci (name_ci) COMMENT 'User list filtered using quick search';
ALTER TABLE user ADD INDEX idx_user_name (name) COMMENT 'User list filtered/ordered by name; User detail filter';

ALTER TABLE usergroup ADD INDEX `idx_usergroup_display_name` (`display_name`) COMMENT 'Usergroup list filtered/ordered by display_name';
ALTER TABLE usergroup ADD INDEX idx_usergroup_name_ci (name_ci) COMMENT 'Usergroup list filtered using quick search';
ALTER TABLE usergroup ADD INDEX idx_usergroup_name (name) COMMENT 'Usergroup list filtered/ordered by name; Usergroup detail filter';

ALTER TABLE host
    MODIFY active_checks_enabled enum('n','y') NOT NULL,
    MODIFY passive_checks_enabled enum('n','y') NOT NULL,
    MODIFY event_handler_enabled enum('n','y') NOT NULL,
    MODIFY notifications_enabled enum('n','y') NOT NULL,
    MODIFY flapping_enabled enum('n','y') NOT NULL,
    MODIFY perfdata_enabled enum('n','y') NOT NULL,
    MODIFY is_volatile enum('n','y') NOT NULL;
ALTER TABLE host_state
    ADD COLUMN normalized_performance_data mediumtext DEFAULT NULL AFTER performance_data,
    ADD COLUMN last_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id' AFTER acknowledgement_comment_id,
    MODIFY is_problem enum('n','y') NOT NULL,
    MODIFY is_handled enum('n','y') NOT NULL,
    MODIFY is_reachable enum('n','y') NOT NULL,
    MODIFY is_flapping enum('n','y') NOT NULL,
    MODIFY is_overdue enum('n','y') NOT NULL,
    MODIFY is_acknowledged enum('n','y','sticky') NOT NULL,
    MODIFY in_downtime enum('n','y') NOT NULL;
ALTER TABLE service
    MODIFY active_checks_enabled enum('n','y') NOT NULL,
    MODIFY passive_checks_enabled enum('n','y') NOT NULL,
    MODIFY event_handler_enabled enum('n','y') NOT NULL,
    MODIFY notifications_enabled enum('n','y') NOT NULL,
    MODIFY flapping_enabled enum('n','y') NOT NULL,
    MODIFY perfdata_enabled enum('n','y') NOT NULL,
    MODIFY is_volatile enum('n','y') NOT NULL;
ALTER TABLE service_state
    ADD COLUMN normalized_performance_data mediumtext DEFAULT NULL AFTER performance_data,
    ADD COLUMN last_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id' AFTER acknowledgement_comment_id,
    MODIFY is_problem enum('n','y') NOT NULL,
    MODIFY is_handled enum('n','y') NOT NULL,
    MODIFY is_reachable enum('n','y') NOT NULL,
    MODIFY is_flapping enum('n','y') NOT NULL,
    MODIFY is_overdue enum('n','y') NOT NULL,
    MODIFY is_acknowledged enum('n','y','sticky') NOT NULL,
    MODIFY in_downtime enum('n','y') NOT NULL;
ALTER TABLE icingadb_instance
    MODIFY responsible enum('n','y') NOT NULL,
    MODIFY icinga2_notifications_enabled enum('n','y') NOT NULL,
    MODIFY icinga2_active_service_checks_enabled enum('n','y') NOT NULL,
    MODIFY icinga2_active_host_checks_enabled enum('n','y') NOT NULL,
    MODIFY icinga2_event_handlers_enabled enum('n','y') NOT NULL,
    MODIFY icinga2_flap_detection_enabled enum('n','y') NOT NULL,
    MODIFY icinga2_performance_data_enabled enum('n','y') NOT NULL;
ALTER TABLE checkcommand_argument
    MODIFY repeat_key enum('n','y') NOT NULL,
    MODIFY required enum('n','y') NOT NULL,
    MODIFY skip_key enum('n','y') NOT NULL;
ALTER TABLE eventcommand_argument
    MODIFY repeat_key enum('n','y') NOT NULL,
    MODIFY required enum('n','y') NOT NULL,
    MODIFY skip_key enum('n','y') NOT NULL;
ALTER TABLE notificationcommand_argument
    MODIFY repeat_key enum('n','y') NOT NULL,
    MODIFY required enum('n','y') NOT NULL,
    MODIFY skip_key enum('n','y') NOT NULL;
ALTER TABLE comment
    MODIFY name varchar(548) NOT NULL COMMENT '255+1+255+1+36, i.e. "host.name!service.name!UUID"',
    MODIFY is_persistent enum('n','y') NOT NULL,
    MODIFY is_sticky enum('n','y') NOT NULL;
ALTER TABLE downtime
    MODIFY name varchar(548) NOT NULL COMMENT '255+1+255+1+36, i.e. "host.name!service.name!UUID"',
    MODIFY is_flexible enum('n','y') NOT NULL,
    MODIFY is_in_effect enum('n','y') NOT NULL;
ALTER TABLE timeperiod
    MODIFY prefer_includes enum('n','y') NOT NULL;
ALTER TABLE user
    MODIFY notifications_enabled enum('n','y') NOT NULL;
ALTER TABLE zone
    MODIFY is_global enum('n','y') NOT NULL;
ALTER TABLE downtime_history
    MODIFY is_flexible enum('n','y') NOT NULL,
    MODIFY has_been_cancelled enum('n','y') NOT NULL;
ALTER TABLE comment_history
    MODIFY is_persistent enum('n','y') NOT NULL,
    MODIFY is_sticky enum('n','y') NOT NULL,
    MODIFY has_been_removed enum('n','y') NOT NULL;
ALTER TABLE acknowledgement_history
    MODIFY author varchar(255) DEFAULT NULL COLLATE utf8mb4_unicode_ci COMMENT 'NULL if ack_set event happened before Icinga DB history recording',
    MODIFY comment text DEFAULT NULL COMMENT 'NULL if ack_set event happened before Icinga DB history recording',
    MODIFY is_sticky enum('n','y') DEFAULT NULL COMMENT 'NULL if ack_set event happened before Icinga DB history recording',
    MODIFY is_persistent enum('n','y') DEFAULT NULL COMMENT 'NULL if ack_set event happened before Icinga DB history recording';

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (2, CURRENT_TIMESTAMP() * 1000);

ALTER TABLE host_state
    MODIFY output mediumtext DEFAULT NULL,
    MODIFY long_output mediumtext DEFAULT NULL,
    MODIFY performance_data mediumtext DEFAULT NULL;

ALTER TABLE state_history
    MODIFY output mediumtext DEFAULT NULL,
    MODIFY long_output mediumtext DEFAULT NULL;

ALTER TABLE service_state
    MODIFY output mediumtext DEFAULT NULL,
    MODIFY long_output mediumtext DEFAULT NULL,
    MODIFY performance_data mediumtext DEFAULT NULL;
