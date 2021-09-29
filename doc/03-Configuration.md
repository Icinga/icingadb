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

## Logging Configuration <a id="configuration-logging"></a>

Configuration of the logging component used by Icinga DB.

Option                   | Description
-------------------------|-----------------------------------------------
level                    | **Optional.** Specifies the default logging level. Can be set to `fatal`, `error`, `warning`, `info` or `debug`. Defaults to `info`.
options                  | **Optional.** Map of component name to logging level in order to set a different logging level for each component instead of the default one. See [logging components](#logging-components) for details.

### Logging Components <a id="logging-components"></a>

Component                | Description
-------------------------|-----------------------------------------------
database                 | Database connection status and queries.
redis                    | Redis connection status and queries.
heartbeat                | Icinga heartbeats received through Redis.
high-availability        | Manages responsibility of Icinga DB instances.
config-sync              | Config object synchronization between Redis and MySQL.
history-sync             | Synchronization of history entries from Redis to MySQL.
runtime-updates          | Runtime updates of config objects after the initial config synchronization.
overdue-sync             | Calculation and synchronization of the overdue status of checkables.
dump-signals             | Dump signals received from Icinga.
