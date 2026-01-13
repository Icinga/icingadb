ALTER TABLE host
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL,
  ALTER COLUMN check_interval TYPE float,
  ALTER COLUMN check_retry_interval TYPE float;

ALTER TABLE host_state
  ALTER COLUMN execution_time TYPE float,
  ALTER COLUMN execution_time SET DEFAULT NULL,
  ALTER COLUMN latency TYPE float,
  ALTER COLUMN latency SET DEFAULT NULL,
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL;

ALTER TABLE service
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL,
  ALTER COLUMN check_interval TYPE float,
  ALTER COLUMN check_retry_interval TYPE float;

ALTER TABLE service_state
  ALTER COLUMN execution_time TYPE float,
  ALTER COLUMN execution_time SET DEFAULT NULL,
  ALTER COLUMN latency TYPE float,
  ALTER COLUMN latency SET DEFAULT NULL,
  ALTER COLUMN check_timeout TYPE float,
  ALTER COLUMN check_timeout SET DEFAULT NULL;
