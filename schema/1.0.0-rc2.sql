ALTER TABLE host_state DROP PRIMARY KEY;
ALTER TABLE host_state ADD COLUMN id binary(20) NOT NULL COMMENT 'host.id' FIRST;
UPDATE host_state SET id = host_id;
ALTER TABLE host_state ADD PRIMARY KEY (id);
ALTER TABLE host_state ADD COLUMN properties_checksum binary(20) AFTER environment_id;
UPDATE host_state SET properties_checksum = 0;
ALTER TABLE host_state MODIFY COLUMN properties_checksum binary(20) COMMENT 'sha1(all properties)' NOT NULL;

ALTER TABLE service_state DROP PRIMARY KEY;
ALTER TABLE service_state ADD COLUMN id binary(20) NOT NULL COMMENT 'service.id' FIRST;
UPDATE service_state SET id = service_id;
ALTER TABLE service_state ADD PRIMARY KEY (id);
ALTER TABLE service_state ADD COLUMN properties_checksum binary(20) AFTER environment_id;
UPDATE service_state SET properties_checksum = 0;
ALTER TABLE service_state MODIFY COLUMN properties_checksum binary(20) COMMENT 'sha1(all properties)' NOT NULL;

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
