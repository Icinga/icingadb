# Upgrading Icinga DB

Specific version upgrades are described below. Please note that version upgrades are incremental.
If you are upgrading across multiple versions, make sure to follow the steps for each of them.

## Upgrading to Icinga DB v1.1.1

Please apply the `1.1.1.sql` upgrade script to your database.
For package installations, you can find this file at `/usr/share/icingadb/schema/mysql/upgrades/` or
`/usr/share/icingadb/schema/pgsql/upgrades/`, depending on your database type.

Note that this upgrade will change the `history` table, which can take some time depending on the size of the table and
the performance of the database. While the upgrade is running, that table will be locked and can't be accessed. This
means that the existing history can't be viewed in Icinga Web and new history entries will be buffered in Redis.

As the daemon checks the schema version, the recommended way to perform the upgrade is to stop the daemon, apply the
schema upgrade and then start the new daemon version. If you want to minimize downtime as much as possible, it is safe
to apply this schema upgrade while the Icinga DB v1.1.0 daemon is still running and then restart the daemon with the
new version. Please keep in mind that depending on the distribution, your package manager may automatically attempt to
restart the daemon when upgrading the package.

## Upgrading to Icinga DB v1.0

**Requirements**

* You need at least Icinga 2 version 2.13.4 to run Icinga DB v1.0.0.

**Database Schema**

* For MySQL databases, please apply the `1.0.0.sql` upgrade script.
  For package installations, you can find this file at `/usr/share/icingadb/schema/mysql/upgrades/`.

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
