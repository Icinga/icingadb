# Icinga DB Migration

![Icinga Logo]

#### Table of Contents

- [About](#about)
- [License](#license)
- [Installation](#installation)
- [Usage](#usage)
- [Support](#support)
- [Contributing](#contributing)

## About

This commandline tool migrates history data from [IDO] to [Icinga DB].
Or, more precisely: from the IDO SQL database to the Icinga DB one.

!!! info

    Everything else is already populated by Icinga DB itself.
    Only the past history data of existing IDO setups
    isn't known to Icinga DB without migration from IDO.

## License

This tool and its documentation are licensed under the terms
of the GNU General Public License Version 2,
you will find a copy of this license in the [LICENSE] file.

## Installation

1. Add the official release repository as described
   in the [installation chapter] of the Icinga DB documentation.
2. Install the package `icingadb-migration`.

## Usage

### Icinga DB

1. Make sure Icinga DB is up, running and writing to its database.
2. Optionally disable Icinga 2's IDO feature.

!!! warning

    Migration will cause duplicate Icinga DB events
    for the period both IDO and Icinga DB are active.
    Disable the IDO feature -- the sooner, the better!
    Or read on while not disabling it yet.
    There is a way to avoid duplicate events.

### Configuration file

Create a YAML file like this somewhere:

```yaml
icinga2:
   # Content of /var/lib/icinga2/icingadb.env
   env: "da39a3ee5e6b4b0d3255bfef95601890afBADHEX"
   # Name of the main Icinga 2 endpoint writing to IDO
   endpoint: master-1
# IDO database
ido:
   type: pgsql
   host: 192.0.2.1
   port: 5432
   database: icinga
   user: icinga
   password: CHANGEME
   # Input time range
   #from: 0
   #to: 2147483647
# Icinga DB database
icingadb:
   type: mysql
   host: 2001:db8::1
   port: 3306
   database: icingadb
   user: icingadb
   password: CHANGEME
```

#### Input time range

By default, everything is migrated. If you wish, you can restrict the input
data's start and/or end by giving `from` and/or `to` under `ido:` as Unix
timestamps (in seconds).

Examples:

* Now: Run in a shell: `date +%s`
* One year ago: Run in a shell: `date -d -1year +%s`
* Icinga DB usage start time: Query the Icinga DB database:
  `SELECT MIN(event_time)/1000 FROM history;`

The latter is useful for the range end to avoid duplicate events.

### Cache directory

Choose a (not necessarily yet existing) directory for Icinga DB Migration's
internal cache. If either there isn't much to migrate or the migration
process won't be interrupted by a reboot (of the machine
Icinga DB migration/database runs on), `mktemp -d` is enough.

### Actual migration

Run:

```shell
icingadb-migrate -c icingadb-migration.yml -t ~/icingadb-migration.tmp
```

In case of an interrupt re-run.

!!! tip

    If there is much to migrate, use e.g. tmux to
    protect yourself against SSH connection losses.

## Support

Check the [project website] for status updates. Join the [community channels]
for questions or ask an Icinga partner for [professional support].

## Contributing

There are many ways to contribute to Icinga -- whether it be sending patches,
testing, reporting bugs, or reviewing and updating the documentation. Every
contribution is appreciated!


[Icinga Logo]: https://icinga.com/wp-content/uploads/2014/06/icinga_logo.png
[IDO]: https://icinga.com/docs/icinga-2/latest/doc/14-features/#ido-database-db-ido
[Icinga DB]: https://icinga.com/docs/icinga-db/latest/doc/01-About/
[LICENSE]: ./LICENSE
[installation chapter]: https://icinga.com/docs/icingadb/latest/doc/02-Installation/
[project website]: https://icinga.com
[community channels]: https://icinga.com/community/
[professional support]: https://icinga.com/support/
