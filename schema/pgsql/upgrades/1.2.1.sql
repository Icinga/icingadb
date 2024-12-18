ALTER TABLE host ALTER COLUMN icon_image_alt TYPE text;
ALTER TABLE service ALTER COLUMN icon_image_alt TYPE text;

COMMENT ON COLUMN endpoint.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN comment.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN notification.properties_checksum IS 'sha1(all properties)';

ALTER TABLE timeperiod_range ALTER COLUMN range_value TYPE text;

ALTER TABLE checkcommand_argument ALTER COLUMN argument_key TYPE varchar(255);
ALTER TABLE eventcommand_argument ALTER COLUMN argument_key TYPE varchar(255);
ALTER TABLE notificationcommand_argument ALTER COLUMN argument_key TYPE varchar(255);

ALTER TABLE checkcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);
ALTER TABLE eventcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);
ALTER TABLE notificationcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (4, extract(epoch from now()) * 1000);
