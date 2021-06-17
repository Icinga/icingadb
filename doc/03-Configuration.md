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

## Cleanup Configuration

This configuration is optional. If configured, the cleanup routine is called every 1 hour by default and 
erases old entries from history tables. In the example the cleanup routine is configured to erase the entries from 
history tables which are older than 10 days.

Option                   | Description
-------------------------|-----------------------------------------------
history                  | **Required.** history table configuration as shown below.

### History Configuration
Option                   | Description
-------------------------|-----------------------------------------------
acknowledgement          | **Optional.** Duration string (E.g: 5h30m40s).
comment                  | **Optional.** Duration string (E.g: 5h30m40s).
database                 | **Optional.** Duration string (E.g: 5h30m40s).
downtime                 | **Optional.** Duration string (E.g: 5h30m40s).
flapping                 | **Optional.** Duration string (E.g: 5h30m40s).
notification             | **Optional.** Duration string (E.g: 5h30m40s).
state                    | **Optional.** Duration string (E.g: 5h30m40s).

The tables `history` and `user_notification_history` are child tables. Hence, deleting records from their parent tables
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
    acknowledgement: 2400h
    comment: 2400h
    downtime: 2400h
    flapping: 2400h
    notification: 2400h
    state: 2400h
```
