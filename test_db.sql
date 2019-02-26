CREATE database icingadb;
CREATE USER 'icingadb'@'127.0.0.1' IDENTIFIED BY 'icingadb';
GRANT ALL PRIVILEGES ON icingadb.* TO 'icingadb'@'127.0.0.1';