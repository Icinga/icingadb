# Migration from IDO

Migrating from the IDO feature to Icinga DB starts by setting up Icinga DB. To
do so, please follow the [installation instructions]. The Icinga DB feature can
be enabled in parallel to the IDO, allowing you to perform the migration while
the IDO is still running.

After setting up Icinga DB, all Icinga objects and their current state should
already show up in Icinga DB Web as this information is synced from Icinga 2.
At this point, the old host and service history is missing in Icinga DB. If it
is desired to keep it, this information has to be migrated explicitly from the
old IDO database. To do so, follow the instructions below.

## History

To migrate history data from the [IDO] database, the Icinga DB Migration
commandline tool is provided. If you have installed Icinga DB from our
packages, it is automatically installed as well.

### Preparing the Configuration

Please take the [example configuration] as a starting point and copy it to the
host you will perform the migration on. The following sections will guide you
through how to adjust it for your needs.

#### Environment ID

Icinga DB allows writing multiple Icinga environments to the same database.
Thus, you have to tell the migration tool for which environment you want to
migrate the history. On each Icinga 2 node that has the Icinga DB feature
enabled, the environment ID is written to the file
`/var/lib/icinga2/icingadb.env`. Please use the contents of this file for the
`env` option in the section `icinga2`.

#### Database Connection

The migration tool needs to access both the IDO and the Icinga DB databases.
Please specify the connection details in the corresponding `ido` and `icingadb`
sections of the configuration.

Both the IDO and Icinga DB support MySQL and PostgreSQL. You can migrate from
and to both types, including from one type to the other.

The fields of the `ido` and `icingadb` sections follow the Icinga DB
[database configuration format](03-Configuration.md#database-configuration),
except for `to` and `from` in `ido`, which are described in the following
documentation section. The `icingadb` section should be identical to the
`database` section of the Icinga DB configuration for most users.

#### Input Time Range

The migration tool allows you to restrict the time range of the history events
to be migrated. This is controlled by the options `from` and `to` in the `ido`
section of the configuration. Both options can be set to Unix timestamps.

It is recommended to set the `to` option to a cutoff time at which the history
in the Icinga DB database switches from migrated events to events written
directly by Icinga DB. If you kept running the IDO in parallel to Icinga DB and
do not do this, there will be duplicate events for the time both were running.

You can query the time of the first history event written by Icinga DB by
running this query in its database:

```
SELECT MIN(event_time)/1000 FROM history;
```

In case you had trouble setting up Icinga DB or this is not the first time you
are setting up Icinga DB, please make sure to double-check this timestamp and
adjust it accordingly if it is not what you expect.

!!! tip

    You can convert between Unix timestamps and a human-readable format using the `date` command:

    * Unix timestamp to readable date: `date -d @1667219820`
    * Current date/time to Unix timestamp: `date +%s`
    * Specific date/time to Unix timestamp: `date -d '2022-01-01 00:00:00' +%s`
    * Relative date/time to Unix timestamp: `date -d '-1 year' +%s`

Similarly, you can use `from` to limit how much old history gets migrated.

### Cache Directory

Choose a (not necessarily yet existing) directory for Icinga DB Migration's
internal cache. If either there isn't much to migrate or the migration
process won't be interrupted by a reboot (of the machine
Icinga DB migration/database runs on), `mktemp -d` is enough.

### Run the Migration

To start the actual migration, execute the following command:

```shell
icingadb-migrate -c icingadb-migration.yml -t ~/icingadb-migration.cache
```

In case this command was interrupted, you can run it again. It will continue
where it left off and reuse the cache if it is still present.

!!! tip

    If there is much to migrate, use e.g. tmux to
    protect yourself against SSH connection losses.

[installation instructions]: 02-Installation.md
[IDO]: https://icinga.com/docs/icinga-2/latest/doc/14-features/#ido-database-db-ido
[example configuration]: icingadb-migration.example.yml
