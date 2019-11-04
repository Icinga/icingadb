-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

CREATE database icingadb;
CREATE USER 'module-dev'@'127.0.0.1' IDENTIFIED BY 'icinga0815!';
GRANT ALL PRIVILEGES ON icingadb.* TO 'module-dev'@'127.0.0.1';