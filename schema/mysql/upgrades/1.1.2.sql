UPDATE icingadb_schema SET timestamp = UNIX_TIMESTAMP(timestamp / 1000) * 1000 WHERE timestamp > 20000000000000000;

ALTER TABLE history ADD INDEX idx_history_event_time_event_type (event_time, event_type) COMMENT 'History filtered/ordered by event_time/event_type';
ALTER TABLE history DROP INDEX idx_history_event_time;
