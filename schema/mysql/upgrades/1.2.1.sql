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

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (6, UNIX_TIMESTAMP() * 1000);
