ALTER TABLE icingadb_instance
  ADD COLUMN icingadb_service_user varchar(255) NOT NULL DEFAULT 'icingadb',
  ADD COLUMN notifications_healthy boolenum DEFAULT NULL,
  ADD COLUMN notifications_discovered_socket_path text;

ALTER TABLE icingadb_instance ALTER COLUMN icingadb_service_user DROP DEFAULT;
