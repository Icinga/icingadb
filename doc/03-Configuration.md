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

## Logging Configuration <a id="configuration-logging"></a>

Configuration of the logging component used by Icinga DB.

Option                   | Description
-------------------------|-----------------------------------------------
level                    | **Required.** Logger default level (debug, info, warn, error, dpanic, panic or fatal). Usually set to `debug` by default.
options                  | **Optional.** Child loggers, with `Icinga Db component` => `logging level` map.<br /> `Icinga Db component`: database, redis, heartbeat, high-availability, config-sync, history, runtime-updates, overdue-sync, dump-signals, delta. <br /> `logging level`: debug, info, warn, error, dpanic, panic or fatal.

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
logging:
  level: debug
  options:
    database: debug
    redis: debug
```
