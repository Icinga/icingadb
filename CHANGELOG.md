# Icinga DB Changelog

## 1.3.0 (2025-04-03)

This is a maintenance release, integrating the container setup directly into Icinga DB.

Most importantly, first-class support for configuring Icinga DB completely via environment variables has been added alongside YAML.
In addition, an optional automatic database schema import has been added, allowing an empty SQL database to be populated.
With these two features, the [docker-icingadb](https://github.com/Icinga/docker-icingadb) repository can now be sunsetted, integrating the `Containerfile` directly into the main repository.

For Docker users of Icinga DB, the following has changed.

- Container images are now pushed to both the [GitHub Container Registry (GHCR)](https://github.com/icinga/icingadb/pkgs/container/icingadb) and [Docker Hub](https://hub.docker.com/r/icinga/icingadb).
- Development images built from the `main` branch are no longer tagged as `main`, but now as `edge`.

The changes are as follows.

* Support loading configuration from both YAML files and environment variables. #831
* Allow database schema import via command line argument flag. #901
* Create and publish container images from the icingadb repository. #912
* Log essential HA events and the startup message regardless of the log level. #920
* Resolve SQL errors with reserved names in `icingadb-migrate` on older PostgreSQL versions. #885

## 1.2.1 (2024-12-18)

This is a maintenance release that addresses HA issues and includes a number of other fixes.

Most prominent, crashes caused by an invalid HA state were investigated and fixed mainly by the following changes.

* Ensure that the crucial HA realization logic is always aborted when its timeout is reached. #800
* Give up the HA leadership role if it seems another node is also active. #825
* Reduce database deadlocks in the HA realization domain with exclusive locking. #830

Other notable changes include the following:

* ACL and database support for Redis®[\*](doc/TRADEMARKS.md#redis). #874, icinga-go-library#50, icinga-go-library#52
* Alter the database schema to allow longer user input. #779, #792, #856
* Mitigate some NULL values for icingadb-migrate. #767
* Retry certain database errors for PostgreSQL. icinga-go-library#59
* Retry Redis® timeout errors for `XREAD`. icinga-go-library#23
* Additional tests were written. #771, #777, #803, #806, #807, #808
* Parts of the code have been moved to our [icinga-go-library](https://github.com/Icinga/icinga-go-library) for use by our other Go daemons. #747
* Update dependencies. [26 times](https://github.com/Icinga/icingadb/pulls?q=is%3Apr+milestone%3A1.2.1+label%3Adependencies)

### Schema

A schema upgrade is available that allows longer user input as listed above.
Please follow the [upgrading documentation](doc/04-Upgrading.md#upgrading-to-icinga-db-v121).

## 1.2.0 (2024-04-11)

This release addresses multiple issues related to fault recoveries,
with a particular focus on retryable database errors that may occur when using Icinga DB with database clusters.

Since there may be a large number of errors that are resolved by retrying after a certain amount of time,
\#698 changed the retry behavior to retry every database-related error for five minutes.
This helps Icinga DB survive network hiccups or more complicated database situations,
such as working with a database cluster.

The latter was specifically addressed in #711 for Galera Clusters on MySQL or MariaDB by configuring `wsrep_sync_wait` on used database sessions.
Galera users should refer to the [Configuration documentation](doc/03-Configuration.md#database-options) for more details.

In summary, the most notable changes are as follows:

* Custom Variables: Render large numbers as-is, not using scientific notation. #657
* Enhance retries for database errors and other failures for up to five minutes. #693, #698, #739, #740
* MySQL/MariaDB: Use strict SQL mode. #699
* MySQL/MariaDB Galera Cluster: Set `wsrep_sync_wait` for cluster-wide causality checks. #711
* Don't crash history sync in the absence of Redis®[\*](doc/TRADEMARKS.md#redis). #725
* Update dependencies. [27 times](https://github.com/Icinga/icingadb/pulls?q=is%3Apr+is%3Amerged+label%3Adependencies+milestone%3A1.2.0)

### Schema

In addition to mandatory schema upgrades, this release includes an optional upgrade that can be applied subsequently.
Details are available in the [Upgrading documentation](doc/04-Upgrading.md#upgrading-to-icinga-db-v120) and #656.

All schema changes are listed below:

* Allow host and service check attempts >= 256. #656
* Composite `INDEX` for the history table to speed up history view in Icinga DB Web. #686
* MySQL/MariaDB: Fix `icingadb_schema.timestamp` not being Unix time. #700
* PostgreSQL: Change `get_sla_ok_percent` to return decimal numbers in SLA overview. #710

## 1.1.1 (2023-08-09)

This release fixes a few crashes in the Icinga DB daemon, addresses some shortcomings in the database schema,
and makes the `icingadb-migrate` tool handle malformed events and other edge-cases more reliably.

* Fix a possible crash when the Icinga 2 heartbeat is lost. #559
* Retry additional non-fatal database errors. #593 #583
* Make heartbeat compatible with Percona XtraDB Cluster. #609
* Write a hint for empty arrays/dicts into `customvar_flat` for Icinga DB Web. #601
* Warn about unknown options in the daemon config file. #605 #631
* Don't log a port number for UNIX socket addresses. #542
* Fix some custom JSON encode functions for `null` values. #612
* Documentation: add TLS options to `icingadb-migrate` example config. #604
* Documentation: Replace `apt-get` with `apt`. #545
* Update dependencies. #548 #549 #588 #589 #590 #594 #595 #596 #598 #599 #603 #632

### Schema

* Allow longer names for notification objects. #584
* Add missing indices to `hostgroup`, `servicegroup`, and `customvar_flat`. #616 #617
* Change sort order of history event types. #626

### icingadb-migrate

* Ignore events that miss crucial information. #551
* Fix a foreign key error for flapping history with `ido.from` set. #554
* Fix a constraint violation for flexible downtimes that never started. #623
* Show an error for unknown options in the config file. #605

## 1.1.0 (2022-11-10)

This release adds a tool for migrating history from IDO. Apart from that,
it reduces RAM usage and includes updated dependencies.

* Add `icingadb-migrate` for migrating IDO history to Icinga DB. #253 #536 #541
* Reduce RAM usage during full sync. #525
* Update dependencies. #522 #524 #533 #534 #540

## 1.0.0 (2022-06-30)

Final release

## 1.0.0 RC2 (2021-11-12)

Second release candidate

## 1.0.0 RC1 (2020-03-13)

Initial release
