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
  VALUES (2, UNIX_TIMESTAMP() * 1000);

ALTER TABLE host_state
    MODIFY output longtext DEFAULT NULL,
    MODIFY long_output longtext DEFAULT NULL,
    MODIFY performance_data longtext DEFAULT NULL;

ALTER TABLE state_history
    ADD COLUMN scheduling_source text DEFAULT NULL AFTER check_source,
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.name + host|service.name + event_time)',
    MODIFY output longtext DEFAULT NULL,
    MODIFY long_output longtext DEFAULT NULL;

ALTER TABLE service_state
    MODIFY output longtext DEFAULT NULL,
    MODIFY long_output longtext DEFAULT NULL,
    MODIFY performance_data longtext DEFAULT NULL;

ALTER TABLE notification_history
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.name + notification.name + type + send_time)';

ALTER TABLE user_notification_history
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(notification_history_id + user_id)',
    MODIFY notification_history_id binary(20) NOT NULL COMMENT 'UUID notification_history.id';

ALTER TABLE history
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.name + event_type + x...) given that sha1(environment.name + x...) = *_history_id',
    MODIFY notification_history_id binary(20) DEFAULT NULL COMMENT 'notification_history.id',
    MODIFY state_history_id binary(20) DEFAULT NULL COMMENT 'state_history.id',
    ADD CONSTRAINT fk_history_acknowledgement_history FOREIGN KEY (acknowledgement_history_id) REFERENCES acknowledgement_history (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_comment_history FOREIGN KEY (comment_history_id) REFERENCES comment_history (comment_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_downtime_history FOREIGN KEY (downtime_history_id) REFERENCES downtime_history (downtime_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_flapping_history FOREIGN KEY (flapping_history_id) REFERENCES flapping_history (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_notification_history FOREIGN KEY (notification_history_id) REFERENCES notification_history (id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_history_state_history FOREIGN KEY (state_history_id) REFERENCES state_history (id) ON DELETE CASCADE;

ALTER TABLE user_notification_history
    ADD CONSTRAINT fk_user_notification_history_notification_history FOREIGN KEY (notification_history_id) REFERENCES notification_history (id) ON DELETE CASCADE;

ALTER TABLE host
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE hostgroup
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE hostgroup_member
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + host_id + hostgroup_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE host_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + host_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE hostgroup_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + hostgroup_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE host_state
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE service
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE servicegroup
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE servicegroup_member
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + servicegroup_id + service_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE service_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + service_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE servicegroup_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + servicegroup_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE service_state
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE endpoint
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE environment
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(Icinga CA public key)';
ALTER TABLE checkcommand
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + type + name)';
ALTER TABLE checkcommand_argument
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + argument_key)';
ALTER TABLE checkcommand_envvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + envvar_key)';
ALTER TABLE checkcommand_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE eventcommand
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + type + name)';
ALTER TABLE eventcommand_argument
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + argument_key)';
ALTER TABLE eventcommand_envvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + envvar_key)';
ALTER TABLE eventcommand_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE notificationcommand
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + type + name)';
ALTER TABLE notificationcommand_argument
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + argument_key)';
ALTER TABLE notificationcommand_envvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + envvar_key)';
ALTER TABLE notificationcommand_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + command_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE comment
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)';
ALTER TABLE downtime
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)';
ALTER TABLE notification
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE notification_user
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notification_id + user_id)';
ALTER TABLE notification_usergroup
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notification_id + usergroup_id)';
ALTER TABLE notification_recipient
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notification_id + (user_id | usergroup_id))';
ALTER TABLE notification_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notification_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE icon_image
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE action_url
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE notes_url
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE timeperiod
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)';
ALTER TABLE timeperiod_range
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + range_id + timeperiod_id)';
ALTER TABLE timeperiod_override_include
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + include_id + timeperiod_id)';
ALTER TABLE timeperiod_override_exclude
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + exclude_id + timeperiod_id)';
ALTER TABLE timeperiod_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + timeperiod_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name + value)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE customvar_flat
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + flatname + flatvalue)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE user
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE usergroup
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE usergroup_member
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + usergroup_id + user_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE user_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + user_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE usergroup_customvar
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + usergroup_id + customvar_id)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE zone
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + name)',
    MODIFY environment_id binary(20) NOT NULL COMMENT 'environment.id';
ALTER TABLE flapping_history
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + "Host"|"Service" + host|service.name + start_time)';
ALTER TABLE acknowledgement_history
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + "Host"|"Service" + host|service.name + set_time)';

/*
 * Schema changes after https://github.com/Icinga/icingadb/pull/403:
 */

ALTER TABLE checkcommand_customvar
    DROP INDEX idx_checkcommand_customvar_command_id,
    DROP INDEX idx_checkcommand_customvar_customvar_id;

ALTER TABLE eventcommand_customvar
    DROP INDEX idx_eventcommand_customvar_command_id,
    DROP INDEX idx_eventcommand_customvar_customvar_id;

ALTER TABLE notificationcommand_customvar
    DROP INDEX idx_notificationcommand_customvar_command_id,
    DROP INDEX idx_notificationcommand_customvar_customvar_id;

ALTER TABLE notification
    RENAME COLUMN command_id TO notificationcommand_id;
ALTER TABLE notification
    MODIFY notificationcommand_id binary(20) NOT NULL COMMENT 'notificationcommand.id';

ALTER TABLE checkcommand_argument
    RENAME COLUMN command_id TO checkcommand_id;
ALTER TABLE checkcommand_argument
    MODIFY checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + checkcommand_id + argument_key)';

ALTER TABLE checkcommand_envvar
    RENAME COLUMN command_id TO checkcommand_id;
ALTER TABLE checkcommand_envvar
    MODIFY checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + checkcommand_id + envvar_key)';

ALTER TABLE checkcommand_customvar
    RENAME COLUMN command_id TO checkcommand_id;
ALTER TABLE checkcommand_customvar
    MODIFY checkcommand_id binary(20) NOT NULL COMMENT 'checkcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + checkcommand_id + customvar_id)';

ALTER TABLE eventcommand_argument
    RENAME COLUMN command_id TO eventcommand_id;
ALTER TABLE eventcommand_argument
    MODIFY eventcommand_id binary(20) NOT NULL COMMENT 'eventcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + eventcommand_id + argument_key)';

ALTER TABLE eventcommand_envvar
    RENAME COLUMN command_id TO eventcommand_id;
ALTER TABLE eventcommand_envvar
    MODIFY eventcommand_id binary(20) NOT NULL COMMENT 'eventcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + eventcommand_id + envvar_key)';

ALTER TABLE eventcommand_customvar
    RENAME COLUMN command_id TO eventcommand_id;
ALTER TABLE eventcommand_customvar
    MODIFY eventcommand_id binary(20) NOT NULL COMMENT 'eventcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + eventcommand_id + customvar_id)';

ALTER TABLE notificationcommand_argument
    RENAME COLUMN command_id TO notificationcommand_id;
ALTER TABLE notificationcommand_argument
    MODIFY notificationcommand_id binary(20) NOT NULL COMMENT 'notificationcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notificationcommand_id + argument_key)';

ALTER TABLE notificationcommand_envvar
    RENAME COLUMN command_id TO notificationcommand_id;
ALTER TABLE notificationcommand_envvar
    MODIFY notificationcommand_id binary(20) NOT NULL COMMENT 'notificationcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notificationcommand_id + envvar_key)';

ALTER TABLE notificationcommand_customvar
    RENAME COLUMN command_id TO notificationcommand_id;
ALTER TABLE notificationcommand_customvar
    MODIFY notificationcommand_id binary(20) NOT NULL COMMENT 'notificationcommand.id',
    MODIFY id binary(20) NOT NULL COMMENT 'sha1(environment.id + notificationcommand_id + customvar_id)';

ALTER TABLE checkcommand_customvar
    ADD INDEX idx_checkcommand_customvar_checkcommand_id (checkcommand_id, customvar_id),
    ADD INDEX idx_checkcommand_customvar_customvar_id (customvar_id, checkcommand_id);

ALTER TABLE eventcommand_customvar
    ADD INDEX idx_eventcommand_customvar_eventcommand_id (eventcommand_id, customvar_id),
    ADD INDEX idx_eventcommand_customvar_customvar_id (customvar_id, eventcommand_id);

ALTER TABLE notificationcommand_customvar
    ADD INDEX idx_notificationcommand_customvar_notificationcommand_id (notificationcommand_id, customvar_id),
    ADD INDEX idx_notificationcommand_customvar_customvar_id (customvar_id, notificationcommand_id);

ALTER TABLE downtime
    ADD COLUMN scheduled_duration bigint unsigned NOT NULL AFTER scheduled_end_time,
    ADD COLUMN duration bigint unsigned NOT NULL COMMENT 'Duration of the downtime: When the downtime is flexible, this is the same as flexible_duration otherwise scheduled_duration' AFTER end_time,
    MODIFY COLUMN flexible_duration bigint unsigned NOT NULL AFTER is_flexible;
UPDATE downtime SET scheduled_duration = scheduled_end_time - scheduled_start_time, duration = (CASE WHEN is_flexible = 'y' THEN flexible_duration ELSE scheduled_end_time - scheduled_start_time END) WHERE scheduled_duration = 0;

ALTER TABLE service_state ADD COLUMN host_id binary(20) NOT NULL COMMENT 'host.id' AFTER id;
UPDATE service_state INNER JOIN service ON service.id = service_state.service_id SET service_state.host_id = service.host_id WHERE service_state.host_id = REPEAT('\0', 20);

ALTER TABLE comment
    ADD INDEX idx_comment_author (author) COMMENT 'Comment list filtered/ordered by author',
    ADD INDEX idx_comment_expire_time (expire_time) COMMENT 'Comment list filtered/ordered by expire_time';

ALTER TABLE downtime
    ADD INDEX idx_downtime_entry_time (entry_time) COMMENT 'Downtime list filtered/ordered by entry_time',
    ADD INDEX idx_downtime_start_time (start_time) COMMENT 'Downtime list filtered/ordered by start_time',
    ADD INDEX idx_downtime_end_time (end_time) COMMENT 'Downtime list filtered/ordered by end_time',
    ADD INDEX idx_downtime_scheduled_start_time (scheduled_start_time) COMMENT 'Downtime list filtered/ordered by scheduled_start_time',
    ADD INDEX idx_downtime_scheduled_end_time (scheduled_end_time) COMMENT 'Downtime list filtered/ordered by scheduled_end_time',
    ADD INDEX idx_downtime_author (author) COMMENT 'Downtime list filtered/ordered by author',
    ADD INDEX idx_downtime_duration (duration) COMMENT 'Downtime list filtered/ordered by duration';

ALTER TABLE service_state
    ADD INDEX idx_service_state_is_problem (is_problem, severity) COMMENT 'Service list filtered by is_problem ordered by severity',
    ADD INDEX idx_service_state_severity (severity) COMMENT 'Service list filtered/ordered by severity',
    ADD INDEX idx_service_state_soft_state (soft_state, last_state_change) COMMENT 'Service list filtered/ordered by soft_state; recently recovered filter',
    ADD INDEX idx_service_state_last_state_change (last_state_change) COMMENT 'Service list filtered/ordered by last_state_change';

ALTER TABLE host_state
    ADD INDEX idx_host_state_is_problem (is_problem, severity) COMMENT 'Host list filtered by is_problem ordered by severity',
    ADD INDEX idx_host_state_severity (severity) COMMENT 'Host list filtered/ordered by severity',
    ADD INDEX idx_host_state_soft_state (soft_state, last_state_change) COMMENT 'Host list filtered/ordered by soft_state; recently recovered filter',
    ADD INDEX idx_host_state_last_state_change (last_state_change) COMMENT 'Host list filtered/ordered by last_state_change';

ALTER TABLE hostgroup
    ADD INDEX idx_hostroup_name (name) COMMENT 'Host/service/host group list filtered by host group name';

ALTER TABLE servicegroup
    ADD INDEX idx_servicegroup_name (name) COMMENT 'Host/service/service group list filtered by service group name';

ALTER TABLE notification
    DROP INDEX idx_host_id,
    DROP INDEX idx_service_id,
    ADD INDEX idx_notification_host_id (host_id),
    ADD INDEX idx_notification_service_id (service_id);

ALTER TABLE zone
    DROP INDEX idx_parent_id,
    ADD INDEX idx_zone_parent_id (parent_id);

ALTER TABLE history
    ADD INDEX idx_history_host_service_id (host_id, service_id, event_time) COMMENT 'Host/service history detail filter';

ALTER TABLE notification_history
    DROP INDEX idx_notification_history_event_time,
    ADD INDEX idx_notification_history_send_time (send_time) COMMENT 'Notification list filtered/ordered by send_time';
