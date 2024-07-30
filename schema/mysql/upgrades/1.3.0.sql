ALTER TABLE host MODIFY COLUMN icon_image_alt TEXT NOT NULL;
ALTER TABLE service MODIFY COLUMN icon_image_alt TEXT NOT NULL;

ALTER TABLE endpoint MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE comment MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';
ALTER TABLE notification MODIFY COLUMN properties_checksum binary(20) NOT NULL COMMENT 'sha1(all properties)';

ALTER TABLE timeperiod_range MODIFY COLUMN range_value text NOT NULL;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (6, UNIX_TIMESTAMP() * 1000);
