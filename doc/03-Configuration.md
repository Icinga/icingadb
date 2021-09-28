# Configuration <a id="configuration"></a>

## Overview <a id="configuration-overview"></a>

The configuration is stored in `/etc/icingadb/icingadb.yml`.
See [config.yml.example](../config.yml.example) for an example configuration.

## Redis Configuration <a id="configuration-redis"></a>

Configuration of the Redis that Icinga writes to.

Option                   | Description
-------------------------|-----------------------------------------------
address                  | **Required.** Redis host:port address.

## Database Configuration <a id="configuration-database"></a>

Configuration of the database used by Icinga DB.

Option                   | Description
-------------------------|-----------------------------------------------
host                     | **Required.** Database host or absolute Unix socket path.
port                     | **Required.** Database port.
database                 | **Required.** Database database.
user                     | **Required.** Database username.
password                 | **Required.** Database password.
