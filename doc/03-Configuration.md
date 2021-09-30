# Configuration <a id="configuration"></a>

## Overview <a id="configuration-overview"></a>

The configuration is stored in `/etc/icingadb/icingadb.yml`.

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
```
