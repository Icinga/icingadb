# Configuration

For package installations, the configuration is stored in `/etc/icingadb/config.yml`.
See [config.example.yml](../config.example.yml) for an example configuration.

## Redis® Configuration

Connection configuration for the Redis® server where Icinga 2 writes its configuration, state and history items.
This is the same connection as configured in the
[Icinga DB feature](https://icinga.com/docs/icinga-2/latest/doc/14-features/#icinga-db) of
the corresponding Icinga 2 node. High availability setups require a dedicated Redis® server per Icinga 2 node and
therefore a dedicated Icinga DB instance that connects to it.

| Option   | Description                                                                                                             |
|----------|-------------------------------------------------------------------------------------------------------------------------|
| host     | **Required.** Host name or address, or absolute Unix socket path.                                                       |
| port     | **Optional.** TCP port. Defaults to `6380` matching the Redis® open source server port in the `icingadb-redis` package. |
| username | **Optional.** Authentication username, requires a `password` being set as well.                                         |
| password | **Optional.** Authentication password. May be used alone or together with a `username`.                                 |
| database | **Optional.** Numerical database identifier, defaults to `0`.                                                           |
| tls      | **Optional.** Whether to use TLS.                                                                                       |
| cert     | **Optional.** Path to TLS client certificate.                                                                           |
| key      | **Optional.** Path to TLS private key.                                                                                  |
| ca       | **Optional.** Path to TLS CA certificate.                                                                               |
| insecure | **Optional.** Whether not to verify the peer.                                                                           |

## Database Configuration

Connection configuration for the database to which Icinga DB synchronizes monitoring data.
This is also the database used in
[Icinga DB Web](https://icinga.com/docs/icinga-db-web) to view and work with the data.
In high availability setups, all Icinga DB instances must write to the same database.

| Option   | Description                                                                                                                                    |
|----------|------------------------------------------------------------------------------------------------------------------------------------------------|
| type     | **Optional.** Either `mysql` (default) or `pgsql`.                                                                                             |
| host     | **Required.** Database host or absolute Unix socket path.                                                                                      |
| port     | **Optional.** Database port. By default, the MySQL or PostgreSQL port, depending on the database type.                                         |
| database | **Required.** Database name.                                                                                                                   |
| user     | **Required.** Database username.                                                                                                               |
| password | **Optional.** Database password.                                                                                                               |
| tls      | **Optional.** Whether to use TLS.                                                                                                              |
| cert     | **Optional.** Path to TLS client certificate.                                                                                                  |
| key      | **Optional.** Path to TLS private key.                                                                                                         |
| ca       | **Optional.** Path to TLS CA certificate.                                                                                                      |
| insecure | **Optional.** Whether not to verify the peer.                                                                                                  |
| options  | **Optional.** List of low-level [database options](#database-options) that can be set to influence some Icinga DB internal default behaviours. |

### Database Options

Each of these configuration options are highly technical with thoroughly considered and tested default values that you
should only change when you exactly know what you are doing. You can use these options to influence the Icinga DB
default behaviour, how it interacts with databases, thus the defaults are usually sufficient for most users and
do not need any manual adjustments.

!!! important

    Do not change the defaults if you do not have to!

| Option                         | Description                                                                                                                                      |
|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| max_connections                | **Optional.** Maximum number of database connections Icinga DB is allowed to open in parallel if necessary. Defaults to `16`.                    |
| max_connections_per_table      | **Optional.** Maximum number of queries Icinga DB is allowed to execute on a single table concurrently. Defaults to `8`.                         |
| max_placeholders_per_statement | **Optional.** Maximum number of placeholders Icinga DB is allowed to use for a single SQL statement. Defaults to `8192`.                         |
| max_rows_per_transaction       | **Optional.** Maximum number of rows Icinga DB is allowed to `SELECT`,`DELETE`,`UPDATE` or `INSERT` in a single transaction. Defaults to `8192`. |
| wsrep_sync_wait                | **Optional.** Enforce [Galera cluster](#galera-cluster) nodes to perform strict cluster-wide causality checks. Defaults to `7`.                  |

## Logging Configuration

Configuration of the logging component used by Icinga DB.

| Option   | Description                                                                                                                                                                                              |
|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| level    | **Optional.** Specifies the default logging level. Can be set to `fatal`, `error`, `warn`, `info` or `debug`. Defaults to `info`.                                                                                                                     |
| output   | **Optional.** Configures the logging output. Can be set to `console` (stderr) or `systemd-journald`. Defaults to systemd-journald when running under systemd, otherwise to console. See notes below for [systemd-journald](#systemd-journald-fields). |
| interval | **Optional.** Interval for periodic logging defined as [duration string](#duration-string). Defaults to `"20s"`.                                                                                                                                      |
| options  | **Optional.** Map of component name to logging level in order to set a different logging level for each component instead of the default one. See [logging components](#logging-components) for details.                                              |

### Logging Components

| Component         | Description                                                                     |
|-------------------|---------------------------------------------------------------------------------|
| config-sync       | Config object synchronization between Redis® and MySQL.                         |
| database          | Database connection status and queries.                                         |
| dump-signals      | Dump signals received from Icinga.                                              |
| heartbeat         | Icinga heartbeats received through Redis®.                                      |
| high-availability | Manages responsibility of Icinga DB instances.                                  |
| history-sync      | Synchronization of history entries from Redis® to MySQL.                        |
| overdue-sync      | Calculation and synchronization of the overdue status of checkables.            |
| redis             | Redis® connection status and queries.                                           |
| retention         | Deletes historical data that exceed their configured retention period.          |
| runtime-updates   | Runtime updates of config objects after the initial config synchronization.     |
| telemetry         | Reporting of Icinga DB status to Icinga 2 via Redis® (for monitoring purposes). |

## Retention

By default, no historical data is deleted, which means that the longer the data is retained,
the more disk space is required to store it. History retention is an optional feature that allows to
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

### Galera Cluster

Icinga DB expects a more consistent behaviour from its database than a
[Galera cluster](https://mariadb.com/kb/en/what-is-mariadb-galera-cluster/) provides by default. To accommodate this,
Icinga DB sets the [wsrep_sync_wait](https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_sync_wait) system
variable for all its database connections. Consequently, strict cluster-wide causality checks are enforced before
executing specific SQL queries, which are determined by the value set in the `wsrep_sync_wait` system variable.
By default, Icinga DB sets this to `7`, which includes `READ, UPDATE, DELETE, INSERT, REPLACE` query types and is
usually sufficient. Unfortunately, this also has the downside that every single Icinga DB query will be blocked until
the cluster nodes resynchronise their states after each executed query, and may result in degraded performance.

However, this does not necessarily have to be the case if, for instance, Icinga DB is only allowed to connect to a
single cluster node at a time. This is the case when a load balancer does not randomly route connections to all the
nodes evenly, but always to the same node until it fails, or if your database cluster nodes have a virtual IP address
fail over assigned. In such situations, you can set the `wsrep_sync_wait` system variable to `0` in the
`/etc/icingadb/config.yml` file to disable it entirely, as Icinga DB doesn't have to wait for cluster
synchronisation then.

### Systemd Journald Fields

When examining the journal with `journalctl`, fields containing additional information are hidden by default.
Setting an appropriate
[`--output` option](https://www.freedesktop.org/software/systemd/man/latest/journalctl.html#Output%20Options)
will include them, such as: `--output verbose` or `--output json`.
For example:

```
journalctl --unit icingadb.service --output verbose
```

All Icinga DB fields are prefixed with `ICINGADB_`, e.g., `ICINGADB_ERROR` for error messages.
