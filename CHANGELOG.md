# Icinga DB Changelog

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
