ALTER TABLE alert_history ADD COLUMN event_summary text;
UPDATE alert_history SET event_summary = COALESCE(NULLIF(LEFT(BTRIM(SPLIT_PART(event_message, E'\n', 1)), 1024), ''), 'no summary');
ALTER TABLE alert_history ALTER COLUMN event_summary SET NOT NULL;
