# Configuration <a id="configuration"></a>

## Overview <a id="configuration-overview"></a>

The configuration is stored in `/etc/icingadb/icingadb.ini`.

## General Configuration <a id="configuration-general"></a>

### Redis Configuration <a id="configuration-general-redis"></a>

Data resource where Icinga 2 is writing monitoring data.

Option                   | Description
-------------------------|-----------------------------------------------
host                     | **Optional.** Redis host, defaults to `127.0.0.1`.
port                     | **Optional.** Redis port, defaults to `6379`.
pool\_size               | **Optional.** Maximum number of socket connections. Defaults to `64`.

### MySQL Configuration <a id="configuration-general-mysql"></a>

Data resource where IcingaDB stores synced data and historical events.

Option                   | Description
-------------------------|-----------------------------------------------
host                     | **Optional.** MySQL host. Defaults to `127.0.0.1`.
port                     | **Optional.** MySQL port. Defaults to `3306`.
database                 | **Optional.** MySQL database. Defaults to `icingadb`.
user                     | **Optional.** MySQL username.
password                 | **Optional.** MySQL password.
max\_open\_conns         | **Optional.** Maximum number of open connections. Defaults to `50`.

### Logging Configuration <a id="configuration-general-logging"></a>

Option                   | Description
-------------------------|-----------------------------------------------
level                    | **Optional.** Specifies the logging level. Can be set to `error`, `warn`, `info` or `debug`. See the [logrus spec](https://github.com/sirupsen/logrus#level-logging).
