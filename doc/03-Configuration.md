# Configuration <a id="configuration"></a>

## Overview <a id="configuration-overview"></a>

The configuration is stored in `/etc/icingadb/config.yml`.
See [config.yml.example](../config.yml.example) for an example configuration.

## Redis Configuration <a id="configuration-redis"></a>

Configuration of the Redis that Icinga writes to.

Option                   | Description
-------------------------|-----------------------------------------------
address                  | **Required.** Redis host:port address or absolute Unix socket path.
password                 | **Optional.** The password to use.
tls                      | **Optional.** Whether to use TLS.
cert                     | **Optional.** Path to TLS client certificate.
key                      | **Optional.** Path to TLS private key.
ca                       | **Optional.** Path to TLS CA certificate.
insecure                 | **Optional.** Whether not to verify the peer.

## Database Configuration <a id="configuration-database"></a>

Configuration of the database used by Icinga DB.

Option                   | Description
-------------------------|-----------------------------------------------
type                     | **Optional.** Either `mysql` (default) or `pgsql`.
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
config-sync              | Config object synchronization between Redis and MySQL.
database                 | Database connection status and queries.
dump-signals             | Dump signals received from Icinga.
heartbeat                | Icinga heartbeats received through Redis.
high-availability        | Manages responsibility of Icinga DB instances.
history-retention        | Deletes historical data that exceed their configured retention period.
history-sync             | Synchronization of history entries from Redis to MySQL.
overdue-sync             | Calculation and synchronization of the overdue status of checkables.
redis                    | Redis connection status and queries.
runtime-updates          | Runtime updates of config objects after the initial config synchronization.

### Duration String <a id="duration-string"></a>

A duration string is a sequence of decimal numbers and a unit suffix, such as `"20s"`.
Valid units are `"ms"`, `"s"`, `"m"` and `"h"`.

## History Retention <a id="configuration-history-retention"></a>

By default, no historical data is deleted, which means that the longer the data is retained, the more disk space is required to store it.
History retention is an optional feature that allows you to limit the number of days that historical data is available for each history category.

| Option  | Description                                                                                                                                                                                                   |
|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| days    | **Optional.** Number of days to retain historical data for all history categories. Use `options` in order to enable retention only for specific categories or to override the retention days configured here. |
| options | **Optional.** Map of history category to number of days to retain its data. Available categories are `acknowledgement`, `comment`, `downtime`, `flapping`, `notification` and `state`.                        |
