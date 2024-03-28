ALTER TABLE state_history ALTER COLUMN check_attempt TYPE uint;

COMMENT ON COLUMN state_history.check_attempt IS NULL;
