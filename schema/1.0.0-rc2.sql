ALTER TABLE host_state DROP PRIMARY KEY;
ALTER TABLE host_state ADD COLUMN id binary(20) NOT NULL FIRST;
UPDATE host_state SET id = host_id;
ALTER TABLE host_state ADD PRIMARY KEY (id);
ALTER TABLE host_state ADD COLUMN properties_checksum binary(20) AFTER environment_id;
UPDATE host_state SET properties_checksum = 0;
ALTER TABLE host_state MODIFY COLUMN properties_checksum binary(20) NOT NULL;

ALTER TABLE service_state DROP PRIMARY KEY;
ALTER TABLE service_state ADD COLUMN id binary(20) NOT NULL FIRST;
UPDATE service_state SET id = service_id;
ALTER TABLE service_state ADD PRIMARY KEY (id);
ALTER TABLE service_state ADD COLUMN properties_checksum binary(20) AFTER environment_id;
UPDATE service_state SET properties_checksum = 0;
ALTER TABLE service_state MODIFY COLUMN properties_checksum binary(20) NOT NULL;
