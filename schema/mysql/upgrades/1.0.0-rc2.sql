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

ALTER TABLE downtime
    ADD COLUMN parent_id binary(20) COMMENT 'For service downtimes, the ID of the host downtime that created this downtime by using the "all_services" flag of the schedule-downtime API.' AFTER triggered_by_id,
    MODIFY COLUMN triggered_by_id binary(20) COMMENT 'The ID of the downtime that triggered this downtime. This is set when creating downtimes on a host or service higher up in the dependency chain using the "child_option" "DowntimeTriggeredChildren" and can also be set manually via the API.';
ALTER TABLE downtime_history
    ADD COLUMN parent_id binary(20) COMMENT 'For service downtimes, the ID of the host downtime that created this downtime by using the "all_services" flag of the schedule-downtime API.' AFTER triggered_by_id,
    MODIFY COLUMN triggered_by_id binary(20) COMMENT 'The ID of the downtime that triggered this downtime. This is set when creating downtimes on a host or service higher up in the dependency chain using the "child_option" "DowntimeTriggeredChildren" and can also be set manually via the API.';

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

ALTER TABLE hostgroup_customvar
    ADD INDEX idx_hostgroup_customvar_hostgroup_id (hostgroup_id, customvar_id),
    ADD INDEX idx_hostgroup_customvar_customvar_id (customvar_id, hostgroup_id);
ALTER TABLE servicegroup_customvar
    ADD INDEX idx_servicegroup_customvar_servicegroup_id (servicegroup_id, customvar_id),
    ADD INDEX idx_servicegroup_customvar_customvar_id (customvar_id, servicegroup_id);
ALTER TABLE checkcommand_customvar
    ADD INDEX idx_checkcommand_customvar_command_id (command_id, customvar_id),
    ADD INDEX idx_checkcommand_customvar_customvar_id (customvar_id, command_id);
ALTER TABLE eventcommand_customvar
    ADD INDEX idx_eventcommand_customvar_command_id (command_id, customvar_id),
    ADD INDEX idx_eventcommand_customvar_customvar_id (customvar_id, command_id);
ALTER TABLE notificationcommand_customvar
    ADD INDEX idx_notificationcommand_customvar_command_id (command_id, customvar_id),
    ADD INDEX idx_notificationcommand_customvar_customvar_id (customvar_id, command_id);
ALTER TABLE notification_customvar
    ADD INDEX idx_notification_customvar_notification_id (notification_id, customvar_id),
    ADD INDEX idx_notification_customvar_customvar_id (customvar_id, notification_id);
ALTER TABLE timeperiod_customvar
    ADD INDEX idx_timeperiod_customvar_timeperiod_id (timeperiod_id, customvar_id),
    ADD INDEX idx_timeperiod_customvar_customvar_id (customvar_id, timeperiod_id);
ALTER TABLE user_customvar
    ADD INDEX idx_user_customvar_user_id (user_id, customvar_id),
    ADD INDEX idx_user_customvar_customvar_id (customvar_id, user_id);
ALTER TABLE usergroup_customvar
    ADD INDEX idx_usergroup_customvar_usergroup_id (usergroup_id, customvar_id),
    ADD INDEX idx_usergroup_customvar_customvar_id (customvar_id, usergroup_id);

ALTER TABLE host
    MODIFY active_checks_enabled enum('n','y') NOT NULL,
    MODIFY passive_checks_enabled enum('n','y') NOT NULL,
    MODIFY event_handler_enabled enum('n','y') NOT NULL,
    MODIFY notifications_enabled enum('n','y') NOT NULL,
    MODIFY flapping_enabled enum('n','y') NOT NULL,
    MODIFY perfdata_enabled enum('n','y') NOT NULL,
    MODIFY is_volatile enum('n','y') NOT NULL;
ALTER TABLE host_state
    ADD COLUMN normalized_performance_data longtext DEFAULT NULL AFTER performance_data,
    ADD COLUMN last_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id' AFTER acknowledgement_comment_id,
    ADD COLUMN scheduling_source text DEFAULT NULL AFTER check_source,
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
    ADD COLUMN normalized_performance_data longtext DEFAULT NULL AFTER performance_data,
    ADD COLUMN last_comment_id binary(20) DEFAULT NULL COMMENT 'comment.id' AFTER acknowledgement_comment_id,
    ADD COLUMN scheduling_source text DEFAULT NULL AFTER check_source,
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
    ADD COLUMN scheduled_by varchar(767) DEFAULT NULL COMMENT 'Name of the ScheduledDowntime which created this Downtime. 255+1+255+1+255, i.e. "host.name!service.name!scheduled-downtime-name"' AFTER end_time,
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
    ADD COLUMN scheduled_by varchar(767) DEFAULT NULL COMMENT 'Name of the ScheduledDowntime which created this Downtime. 255+1+255+1+255, i.e. "host.name!service.name!scheduled-downtime-name"' AFTER end_time,
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
    MODIFY output longtext DEFAULT NULL,
    MODIFY long_output longtext DEFAULT NULL,
    MODIFY performance_data longtext DEFAULT NULL;

ALTER TABLE state_history
    ADD COLUMN scheduling_source text DEFAULT NULL AFTER check_source,
    MODIFY output longtext DEFAULT NULL,
    MODIFY long_output longtext DEFAULT NULL;

ALTER TABLE service_state
    MODIFY output longtext DEFAULT NULL,
    MODIFY long_output longtext DEFAULT NULL,
    MODIFY performance_data longtext DEFAULT NULL;

ALTER TABLE user_notification_history
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(notification_history_id + user_id)';

ALTER TABLE history
    ADD CONSTRAINT fk_history_acknowledgement_history FOREIGN KEY (acknowledgement_history_id) REFERENCES acknowledgement_history (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_comment_history FOREIGN KEY (comment_history_id) REFERENCES comment_history (comment_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_downtime_history FOREIGN KEY (downtime_history_id) REFERENCES downtime_history (downtime_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_flapping_history FOREIGN KEY (flapping_history_id) REFERENCES flapping_history (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_notification_history FOREIGN KEY (notification_history_id) REFERENCES notification_history (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_state_history FOREIGN KEY (state_history_id) REFERENCES state_history (id) ON DELETE CASCADE;

ALTER TABLE user_notification_history
    ADD CONSTRAINT fk_user_notification_history_notification_history FOREIGN KEY (notification_history_id) REFERENCES notification_history (id) ON DELETE CASCADE;
