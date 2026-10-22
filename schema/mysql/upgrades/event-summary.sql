ALTER TABLE alert_history ADD COLUMN event_summary longtext;
UPDATE alert_history SET event_summary = COALESCE(NULLIF(LEFT(TRIM(SUBSTRING_INDEX(event_message, '\n', 1)), 1024), ''), 'no summary');
ALTER TABLE alert_history MODIFY COLUMN event_summary longtext NOT NULL;
