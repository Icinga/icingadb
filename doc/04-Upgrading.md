# Upgrading Icinga DB

Some Icinga DB upgrades require manual intervention, others do not. If you need to intervene, the release notes will
point you to the specific upgrade section on this page.

Please note that version upgrades are incremental. If you are upgrading across multiple versions, make sure to follow
the steps for each of them. For example, when upgrading from version 1.1.0 to 1.2.0, follow all instructions for
upgrading to 1.1.1, then all for 1.2.0, including schema upgrades.

## Database Schema Upgrades

Certain Icinga DB version upgrades require a database schema upgrade. If the upgrade section of the specific Icinga DB
release mentions a schema upgrade, this section will guide you through the process of applying the schema upgrade.

First, stop the Icinga DB daemon. If you have an HA setup, stop all Icinga DB instances.

```
systemctl stop icingadb
```

Locate the required schema upgrade files in `/usr/share/icingadb/schema/mysql/upgrades/` for MySQL/MariaDB or in
`/usr/share/icingadb/schema/pgsql/upgrades/` for PostgreSQL. The schema upgrade files are named after the new Icinga DB
release and are mentioned in the specific section below. If you have skipped multiple Icinga DB releases, apply all
schema versions in their order, starting with the earliest release.

The following commands would apply a sample version 1.2.3 schema upgrade to the `icingadb` database as the `icingadb`
user. Please modify them for your setup and the schema upgrade you want to apply.

!!! important

    For PostgreSQL, the schema upgrade must be applied by the `icingadb` PostgreSQL user, since this user owns the
    current tables and would own any new table created by the schema upgrade.
    If you are unsure whether your PostgreSQL user is named `icingadb`, as stated in the installation section,
    you can list the _Owner_ for each table in the `icingadb` database via `\d` in `psql`.

    ```
    $ psql -U postgres icingadb -c '\d'
                          List of relations
     Schema |             Name              |   Type   |  Owner
    --------+-------------------------------+----------+----------
     public | acknowledgement_history       | table    | icingadb
     public | action_url                    | table    | icingadb
     public | checkcommand                  | table    | icingadb
    [ . . . ]
    ```

    This shortened output shows that `icingadb` is the _Owner_ and needs to be set as `-U icingadb` in the following
    upgrade command.

* MySQL/MariaDB:
  ```
  mysql -u icingadb -p icingadb < /usr/share/icingadb/schema/mysql/upgrades/1.2.3.sql
  ```
* PostgreSQL:
  ```
  psql -U icingadb icingadb < /usr/share/icingadb/schema/pgsql/upgrades/1.2.3.sql
  ```

Afterwards, restart Icinga DB. If you have an HA setup, restart all Icinga DB instances.

```
systemctl start icingadb
```

## Upgrading to Icinga DB v1.4.0

### Requirements

Version 1.4.0 of Icinga DB is released alongside Icinga 2.15.0 and Icinga DB Web 1.2.0. A change to the internal
communication API requires these updates to be applied together. To put it simply, Icinga DB 1.4.0 needs Icinga 2.15.0
or later.

The minimum required versions of the MySQL/MariaDB server is increased to support Recursive Common Table Expressions.
Technical information is available in [#947](https://github.com/Icinga/icingadb/issues/947).

* MySQL must be version 8.0 or later.
* MariaDB must be version 10.2.2 or later.

Ensure that the new requirements are met before updating Icinga DB.

### Schema

The upgrade script `1.4.0.sql` must be applied as described in the [schema upgrade section](#database-schema-upgrades).

## Upgrading to Icinga DB v1.2.1

Please apply the `1.2.1.sql` upgrade script to your database. For package installations, you can find this file at
`/usr/share/icingadb/schema/mysql/upgrades/` or `/usr/share/icingadb/schema/pgsql/upgrades/`, depending on your
database vendor.

## Upgrading to Icinga DB v1.2.0

Please apply the `1.2.0.sql` upgrade script to your database. For package installations, you can find this file at
`/usr/share/icingadb/schema/mysql/upgrades/` or `/usr/share/icingadb/schema/pgsql/upgrades/`, depending on your
database vendor.

As the daemon checks the schema version, the recommended way to perform the upgrade is to stop the daemon, apply the
schema upgrade and then start the new daemon version. If you want to minimize downtime as much as possible, it is safe
to apply this schema upgrade while the Icinga DB v1.1.1 daemon is still running and then restart the daemon with the
new version. Please keep in mind that depending on the distribution, your package manager may automatically attempt to
restart the daemon when upgrading the package.

!!! warning

    With MySQL and MariaDB, a locking issue can occur if the schema upgrade is applied while the history view is
    accessed in Icinga DB Web. This can result in the upgrade being delayed unnecessarily and blocking other queries.
    Please see [unblock history tables](#unblock-history-tables) for how to detect and resolve this situation.

### Upgrading the state_history Table

This release includes fixes for hosts and services reaching check attempt 256. However, on existing installations,
the schema upgrade required to fix the history tables isn't automatically applied by `1.2.0.sql` as a rewrite of the
whole `state_history` table is required. This can take a lot of time depending on the history size and the performance
of the database. During this time that table will be locked exclusively and can't be accessed otherwise. This means that
the existing history can't be viewed in Icinga Web and new history entries will be buffered in Redis®.

There is a separate upgrade script `optional/1.2.0-history.sql` to perform the rewrite of the `state_history` table.
This allows you to postpone part of the upgrade to a longer maintenance window in the future, or skip it entirely
if you deem this safe for your installation.

!!! warning

    Until `optional/1.2.0-history.sql` is applied, you'll have to lower `max_check_attempts` to 255 or less, otherwise
    Icinga DB will crash with a database error if hosts/services reach check attempt 256. If you need to lower
    `max_check_attempts` but want to keep the same timespan from an outage to a hard state, you can raise
    `retry_interval` instead so that `max_check_attempts * retry_interval` stays the same.

If you apply it, be sure that `1.2.0.sql` was already applied before. Do not interrupt it! At best use tmux/screen not
to lose your SSH session.

### Unblock History Tables

!!! info

    You don't need to read this section if you are using PostgreSQL. This applies to MySQL/MariaDB users only.

In order to fix a loading performance issue of the history view in Icinga DB Web, this upgrade script adds an
appropriate index on the `history` table. Creating this new index normally takes place without blocking any other
queries. However, this may hang for a relatively considerable time, blocking all Icinga DB queries on all`*_history`
tables and the `history` table inclusively if there is an ongoing, long-running query on the `history` table. One way
of causing this to happen is if an Icinga Web user accesses the `icingadb/history` view just before you are running
this script. Depending on how many entries you have in the history table, Icinga DB Web may take quite a long time to
load, until your web servers timeout (if any) kicks in.

When you observe that the upgrade script has been taking unusually long (`> 60s`) to complete, you can perform the
following analysis on another console and unblock it if necessary. It is important to note though that the script may
need some time to perform the reindexing on the `history` table even if it is not blocked. Nonetheless, you can use the
`show processlist` command to determine whether an excessive number of queries have been stuck in a waiting state.

```
MariaDB [icingadb]> show processlist;
+------+-----+-----+----------+-----+------+---------------------------------+------------------------------------+-----+
| Id   | ... | ... | db       | ... | Time | State                           | Info                               | ... |
+------+-----+-----+----------+-----+------+---------------------------------+------------------------------------+-----+
| 1475 | ... | ... | icingadb | ... | 1222 | Waiting for table metadata lock | INSERT INTO "notification_history" | ... |
| 1485 | ... | ... | icingadb | ... | 1262 | Creating sort index             | SELECT history.id, history....     | ... |
| 1494 | ... | ... | icingadb | ... | 1224 | Waiting for table metadata lock | ALTER TABLE history ADD INDEX ...  | ... |
| 1499 | ... | ... | icingadb | ... | 1215 | Waiting for table metadata lock | INSERT INTO "notification_history" | ... |
| 1500 | ... | ... | icingadb | ... | 1215 | Waiting for table metadata lock | INSERT INTO "state_history" ...    | ... |
| ...  | ... | ... |   ...    | ... | ...  |               ...               |                 ...                | ... |
+------+-----+-----+----------+-----+------+---------------------------------+------------------------------------+-----+
```

In the above output are way too many Icinga DB queries, including the `ALTER TABLE history ADD INDEX` query from the
upgrade script, waiting for a metadata lock, they are just minimised to the bare essentials. Unfortunately, only one of
these queries is holding the `table metadata lock` that everyone else is now waiting for, which in this case is a
`SELECT` statement initiated by Icinga DB Web in the `icingadb/history` view, which takes an unimaginably long time.
Note that there might be multiple `SELECT` statements started before the upgrade script in your case when the history
view of your Icinga DB Web is opened by different Icinga Web users at the same time.

You can now either just wait for the `SELECT` statements to finish by themselves and let them block the upgrade script
and all Icinga DB queries on all `*_history` tables or forcibly terminate them and let the remaining queries do their
work. In this case, cancelling that one blocking `SELECT` query will let the upgrade script continue normally without
blocking any other queries.
```
MariaDB [icingadb]> kill 1485;
```
In case you are insecure about which Icinga DB Web queries are blocking, you may simply cancel all long-running
`SELECT` statements listed with `show processlist` (see column `Time`). Cancelling a `SELECT` query will neither
crash Icinga DB nor corrupt your database, so feel free to abort every single one of them matching the Icinga DB
database (see column `db`).

## Upgrading to Icinga DB v1.1.1

Please apply the `1.1.1.sql` upgrade script to your database.
For package installations, you can find this file at `/usr/share/icingadb/schema/mysql/upgrades/` or
`/usr/share/icingadb/schema/pgsql/upgrades/`, depending on your database type.

Note that this upgrade will change the `history` table, which can take some time depending on the size of the table and
the performance of the database. While the upgrade is running, that table will be locked and can't be accessed. This
means that the existing history can't be viewed in Icinga Web and new history entries will be buffered in Redis®.

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

Icinga DB RC2 is a complete rewrite compared to RC1. Because of this, a lot has changed in the Redis® and database
schema, which is why they have to be deleted and recreated. The configuration file has changed from `icingadb.ini`
to `config.yml`. Instead of the INI format, we are now using YAML and have introduced more configuration options. We
have also changed the packages of `icingadb-redis`, which is why the Redis® CLI commands are now prefixed with `icingadb`
instead of just `icinga`, i.e. the Redis® CLI is now accessed via `icingadb-redis-cli`.

Please follow the steps below to upgrade to Icinga DB RC2:

1. Stop Icinga 2 and Icinga DB.
2. Flush your Redis® instances using `icinga-redis-cli flushall` (note the `icinga` prefix as we did not
   upgrade `icingadb-redis` yet) and stop them afterwards.
3. Upgrade Icinga 2 to version 2.13.2 or newer.
4. Remove the `icinga-redis` package where installed as it may conflict with `icingadb-redis`.
5. Install Redis® (`icingadb-redis`) on your primary Icinga 2 nodes to version 6.2.6 or newer.
6. Upgrade Icinga DB to RC2.
7. Drop the Icinga DB MySQL database and recreate it using the provided schema.
8. Start Redis®, Icinga 2 and Icinga DB.
