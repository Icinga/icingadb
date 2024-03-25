# Configuration

The configuration is stored in `/etc/icingadb/config.yml`.
See [config.example.yml](../config.example.yml) for an example configuration.

## Redis Configuration

Connection configuration for the Redis server where Icinga 2 writes its configuration, state and history items.
This is the same connection as configured in the
[Icinga DB feature](https://icinga.com/docs/icinga-2/latest/doc/14-features/#icinga-db) of
the corresponding Icinga 2 node. High availability setups require a dedicated Redis server per Icinga 2 node and
therefore a dedicated Icinga DB instance that connects to it.

| Option   | Description                                                                                                                        |
|----------|------------------------------------------------------------------------------------------------------------------------------------|
| host     | **Required.** Redis host or absolute Unix socket path.                                                                             |
| port     | **Optional.** Redis port. Defaults to `6380` since the Redis server provided by the `icingadb-redis` package listens on that port. |
| password | **Optional.** The password to use.                                                                                                 |
| tls      | **Optional.** Whether to use TLS.                                                                                                  |
| cert     | **Optional.** Path to TLS client certificate.                                                                                      |
| key      | **Optional.** Path to TLS private key.                                                                                             |
| ca       | **Optional.** Path to TLS CA certificate.                                                                                          |
| insecure | **Optional.** Whether not to verify the peer.                                                                                      |

## Database Configuration

Connection configuration for the database to which Icinga DB synchronizes monitoring data.
This is also the database used in
[Icinga DB Web](https://icinga.com/docs/icinga-db-web) to view and work with the data.
In high availability setups, all Icinga DB instances must write to the same database.

| Option   | Description                                                                                            |
|----------|--------------------------------------------------------------------------------------------------------|
| type     | **Optional.** Either `mysql` (default) or `pgsql`.                                                     |
| host     | **Required.** Database host or absolute Unix socket path.                                              |
| port     | **Optional.** Database port. By default, the MySQL or PostgreSQL port, depending on the database type. |
| database | **Required.** Database name.                                                                           |
| user     | **Required.** Database username.                                                                       |
| password | **Optional.** Database password.                                                                       |
| tls      | **Optional.** Whether to use TLS.                                                                      |
| cert     | **Optional.** Path to TLS client certificate.                                                          |
| key      | **Optional.** Path to TLS private key.                                                                 |
| ca       | **Optional.** Path to TLS CA certificate.                                                              |
| insecure | **Optional.** Whether not to verify the peer.                                                          |

## Logging Configuration

Configuration of the logging component used by Icinga DB.

| Option   | Description                                                                                                                                                                                              |
|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| level    | **Optional.** Specifies the default logging level. Can be set to `fatal`, `error`, `warn`, `info` or `debug`. Defaults to `info`.                                                                        |
| output   | **Optional.** Configures the logging output. Can be set to `console` (stderr) or `systemd-journald`. If not set, logs to systemd-journald when running under systemd, otherwise stderr.                  |
| interval | **Optional.** Interval for periodic logging defined as [duration string](#duration-string). Defaults to `"20s"`.                                                                                         |
| options  | **Optional.** Map of component name to logging level in order to set a different logging level for each component instead of the default one. See [logging components](#logging-components) for details. |

### Logging Components

| Component         | Description                                                                    |
|-------------------|--------------------------------------------------------------------------------|
| config-sync       | Config object synchronization between Redis and MySQL.                         |
| database          | Database connection status and queries.                                        |
| dump-signals      | Dump signals received from Icinga.                                             |
| heartbeat         | Icinga heartbeats received through Redis.                                      |
| high-availability | Manages responsibility of Icinga DB instances.                                 |
| history-sync      | Synchronization of history entries from Redis to MySQL.                        |
| overdue-sync      | Calculation and synchronization of the overdue status of checkables.           |
| redis             | Redis connection status and queries.                                           |
| retention         | Deletes historical data that exceed their configured retention period.         |
| runtime-updates   | Runtime updates of config objects after the initial config synchronization.    |
| telemetry         | Reporting of Icinga DB status to Icinga 2 via Redis (for monitoring purposes). |

## Retention

By default, no historical data is deleted, which means that the longer the data is retained,
the more disk space is required to store it.  History retention is an optional feature that allows to
limit the number of days that historical data is available for each history category.
There are separate options for the full history tables used to display history information in the web interface and
SLA tables which store the minimal information required for SLA reporting,
allowing to keep this information for longer with a smaller storage footprint.

| Option       | Description                                                                                                                                                                                                   |
|--------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| history-days | **Optional.** Number of days to retain historical data for all history categories. Use `options` in order to enable retention only for specific categories or to override the retention days configured here. |
| sla-days     | **Optional.** Number of days to retain historical data for SLA reporting.                                                                                                                                     |
| interval     | **Optional.** Interval for periodically cleaning up the historical data, defined as [duration string](#duration-string). Defaults to `"1h"`.                                                                  |
| count        | **Optional.** Number of old historical data a single query can delete in a `"DELETE FROM ... LIMIT count"` manner. Defaults to `5000`.                                                                        |
| options      | **Optional.** Map of history category to number of days to retain its data. Available categories are `acknowledgement`, `comment`, `downtime`, `flapping`, `notification`, `sla` and `state`.                 |

## Appendix

### Duration String

A duration string is a sequence of decimal numbers and a unit suffix, such as `"20s"`.
Valid units are `"ms"`, `"s"`, `"m"` and `"h"`.
