# Configuration <a id="configuration"></a>

## Overview <a id="configuration-overview"></a>

The configuration is stored in `/etc/icingadb/icingadb.yml`.
See [config.yml.example](../config.yml.example) for an example configuration.

## Redis Configuration <a id="configuration-redis"></a>

Configuration of the Redis that Icinga writes to.

Option                   | Description
-------------------------|-----------------------------------------------
address                  | **Required.** Redis host:port address.
tls                      | **Optional.** Whether to use TLS.
cert                     | **Optional.** Path to TLS client certificate.
key                      | **Optional.** Path to TLS private key.
ca                       | **Optional.** Path to TLS CA certificate.
insecure                 | **Optional.** Whether not to verify the peer.

## Database Configuration <a id="configuration-database"></a>

Configuration of the database used by Icinga DB.

Option                   | Description
-------------------------|-----------------------------------------------
host                     | **Required.** Database host or absolute Unix socket path.
port                     | **Required.** Database port.
database                 | **Required.** Database database.
user                     | **Required.** Database username.
password                 | **Required.** Database password.
tls                      | **Optional.** Whether to use TLS.
cert                     | **Optional.** Path to TLS client certificate.
key                      | **Optional.** Path to TLS private key.
ca                       | **Optional.** Path to TLS CA certificate.
insecure                 | **Optional.** Whether not to verify the peer.

## Logging Configuration <a id="configuration-logging"></a>

Configuration of the logging component used by Icinga DB.

Option                   | Description
-------------------------|-----------------------------------------------
level                    | **Optional.** Specifies the default logging level. Can be set to `fatal`, `error`, `warn`, `info` or `debug`. Defaults to `info`.
output                   | **Optional.** Configures the logging output. Can be set to `console` (stderr) or `systemd-journald`. If not set, logs to systemd-journald when running under systemd, otherwise stderr.
interval                 | **Optional.** Interval for periodic logging defined as [duration string](#duration-string). Defaults to `"20s"`.
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

### Duration String <a id="duration-string"></a>

A duration string is a sequence of decimal numbers and a unit suffix, such as `"20s"`.
Valid units are `"ms"`, `"s"`, `"m"` and `"h"`.
