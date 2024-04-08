UPDATE icingadb_schema SET timestamp = UNIX_TIMESTAMP(timestamp / 1000) * 1000 WHERE timestamp > 20000000000000000;

ALTER TABLE history ADD INDEX idx_history_event_time_event_type (event_time, event_type) COMMENT 'History filtered/ordered by event_time/event_type';
ALTER TABLE history DROP INDEX idx_history_event_time;

ALTER TABLE host_state MODIFY COLUMN check_attempt int unsigned NOT NULL;

ALTER TABLE service_state MODIFY COLUMN check_attempt int unsigned NOT NULL;

ALTER TABLE state_history MODIFY COLUMN check_attempt tinyint unsigned NOT NULL COMMENT 'optional schema upgrade not applied yet, see https://icinga.com/docs/icinga-db/latest/doc/04-Upgrading/#upgrading-to-icinga-db-v112';

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (5, UNIX_TIMESTAMP() * 1000);
