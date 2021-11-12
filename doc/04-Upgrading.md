# Upgrading Icinga DB <a id="upgrading"></a>

## Upgrading to Icinga DB RC2

Icinga DB RC2 is a complete rewrite compared to RC1. Because of this, a lot has changed in the Redis and database
schema, which is why they have to be deleted and recreated. The configuration file has changed from `icingadb.ini`
to `config.yml`. Instead of the INI format, we are now using YAML and have introduced more configuration options. We
have also changed the packages of `icingadb-redis`, which is why the Redis CLI commands are now prefixed with `icingadb`
instead of just `icinga`, i.e. the Redis CLI is now accessed via `icingadb-redis-cli`.

Please follow the steps below to upgrade to Icinga DB RC2:

1. Stop Icinga 2 and Icinga DB.
2. Flush your Redis instances using `icinga-redis-cli flushall` (note the `icinga` prefix as we did not
   upgrade `icingadb-redis` yet) and stop them afterwards.
3. Upgrade Icinga 2 to version 2.13.2 or newer.
4. Remove the `icinga-redis` package where installed as it may conflict with `icingadb-redis`.
5. Install Icinga DB Redis (`icingadb-redis`) on your primary Icinga 2 nodes to version 6.2.6 or newer.
6. Upgrade Icinga DB to RC2.
7. Drop the Icinga DB MySQL database and recreate it using the provided schema.
8. Start Icinga DB Redis, Icinga 2 and Icinga DB.
