ALTER TABLE icingadb_instance
  ADD COLUMN icingadb_service_user varchar(255) NOT NULL DEFAULT 'icingadb',
  ADD COLUMN notifications_healthy enum('n', 'y') DEFAULT NULL,
  ADD COLUMN notifications_discovered_socket_path mediumtext;

ALTER TABLE icingadb_instance MODIFY COLUMN icingadb_service_user varchar(255) NOT NULL;
