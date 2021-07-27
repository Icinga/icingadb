# Configuration <a id="configuration"></a>

## Overview <a id="configuration-overview"></a>

The configuration is stored in `/etc/icingadb/icingadb.yml`.

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

## Cleanup Configuration <a id="configuration-cleanup"></a>

This configuration is optional. If configured, the cleanup routine for the configured tables will be called every 1 hour by default and 
the entries in the history tables older than retention period is erased. In the example the cleanup routine is configured to erase the entries from 
history tables which are older than 10 days.

Option                   | Description
-------------------------|-----------------------------------------------
history                  | **Optional.** history table configuration as shown below.
options                  | **Optional.** Interval and count.<br /> `Interval`:  time.Duration, at which the cleanup routine for each history table is repeated. <br /> `count`: Number of history records to delete at each cleanup from a table (int).

### History Configuration
Option                   | Description
-------------------------|-----------------------------------------------
acknowledgement          | **Optional.** Days (uint).
comment                  | **Optional.** Days (uint).
database                 | **Optional.** Days (uint).
downtime                 | **Optional.** Days (uint).
flapping                 | **Optional.** Days (uint).
notification             | **Optional.** Days (uint).
state                    | **Optional.** Days (uint).

The tables `history` and `user_notification_history` are child tables and have cascade-on-delete configured. Hence, deleting records from their parent tables
automatically deletes the subsequent records in these tables as well. Therefore, they should not be configured in 
cleanup configuration

## Example Configuration <a id="configuration-example"></a>

```yaml
database:
  host: icingadb
  port: 3306
  database: icingadb
  user: icingadb
  password: icingadb
redis:
  address: redis:6380
cleanup:
  history:
    acknowledgement: 10
    comment: 10
    downtime: 10
    flapping: 10
    notification: 10
    state: 10
  options:
    interval: 1h
    count: 5000
```
