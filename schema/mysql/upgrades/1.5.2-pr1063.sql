ALTER TABLE host
  MODIFY COLUMN check_timeout float DEFAULT NULL,
  MODIFY COLUMN check_interval float NOT NULL,
  MODIFY COLUMN check_retry_interval float NOT NULL;

ALTER TABLE host_state
  MODIFY COLUMN execution_time float DEFAULT NULL,
  MODIFY COLUMN latency float DEFAULT NULL,
  MODIFY COLUMN check_timeout float DEFAULT NULL;

ALTER TABLE service
  MODIFY COLUMN check_timeout float DEFAULT NULL,
  MODIFY COLUMN check_interval float NOT NULL,
  MODIFY COLUMN check_retry_interval float NOT NULL;

ALTER TABLE service_state
  MODIFY COLUMN execution_time float DEFAULT NULL,
  MODIFY COLUMN latency float DEFAULT NULL,
  MODIFY COLUMN check_timeout float DEFAULT NULL;
