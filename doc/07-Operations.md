# Operations

This section is a loose collection of various topics and external references for running Icinga DB on a day-to-day basis.
It covers topics such as self-monitoring, backups, and specifics of third-party components.

## Monitor Icinga DB

It is strongly recommended to monitor the monitoring.

There is a built-in [`icingadb` check command](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#icingadb) in the Icinga 2 ITL.
It covers several potential errors, including operations that take too long or invalid high availability scenarios.
Even if the Icinga DB has crashed, checks will still run and Icinga 2 would generate notifications.

In addition, both the Redis® and the relational database should be monitored.
There are predefined check commands in the ITL to choose from.

- [`redis`](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#redis)
- [`mysql`](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#mysql)
- [`mysql_health`](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#mysql_health)
- [`postgres`](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#postgres)

A simpler approach would be to check if the processes are running, e.g.,
with [`proc`](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#procs) or
[`systemd`](https://icinga.com/docs/icinga-2/latest/doc/10-icinga-template-library/#systemd).

## Backups

There are only two things to back up in Icinga DB.

1. The configuration file in `/etc/icingadb` and
2. the relational database, using `mysqldump`, `mariadb-dump` or `pg_dump`.

!!! warning

    When creating a database dump for MySQL or MariaDB with `mysqldump` or `mariadb-dump`,
    use the [`--single-transaction` command line argument flag](https://dev.mysql.com/doc/refman/8.4/en/mysqldump.html#option_mysqldump_single-transaction)
    to not lock the whole database while the backup is running.

## Third-Party Configuration

Icinga DB relies on external components to work.
The following collection is based on experience.
It is a target for continuous improvement.

### MySQL and MariaDB

#### `max_allow_packets`

The `max_allow_packets` system variable limits the size of messages between MySQL/MariaDB servers and clients.
More information is available in
[MySQL's "Replication and max_allowed_packet" documentation section](https://dev.mysql.com/doc/refman/8.4/en/replication-features-max-allowed-packet.html),
[MySQL's variable documentation](https://dev.mysql.com/doc/refman/8.4/en/server-system-variables.html#sysvar_max_allowed_packet) and
[MariaDB's variable documentation](https://mariadb.com/kb/en/server-system-variables/#max_allowed_packet).

The database configuration should have `max_allow_packets` set to at least `64M`.

#### Amazon RDS for MySQL

When importing the MySQL schema into Amazon RDS for MySQL, the following may occur.

```
Error 1419: You do not have the SUPER privilege and binary logging is enabled (you *might* want to use the less safe log_bin_trust_function_creators variable)
```

This error can be mitigated by creating and modifying a custom DB parameter group as described in the related [AWS Knowledge Center article](https://repost.aws/knowledge-center/rds-mysql-functions).

#### Galera Cluster

Starting with Icinga DB version 1.2.0, Galera support has been added to the Icinga DB daemon.
Its specific database configuration is described in the [Galera configuration section](03-Configuration.md#galera-cluster).

As mentioned in [MariaDB's known Galera cluster limitations](https://mariadb.com/kb/en/mariadb-galera-cluster-known-limitations/),
transactions are limited in both amount of rows (128K) and size (2GiB).
A busy Icinga setup can cause Icinga DB to create transactions that exceed these limits with the default configuration.

If you get an error like `Error 1105 (HY000): Maximum writeset size exceeded`
and your Galera node logs something like `WSREP: transaction size limit (2147483647) exceeded`,
decrease the values of `max_placeholders_per_statement` and `max_rows_per_transaction` in Icinga DB's
[Database Options](https://icinga.com/docs/icinga-db/latest/doc/03-Configuration/#database-options).

### Redis®
The official [Redis® administration documentation](https://redis.io/docs/latest/operate/oss_and_stack/management/admin/) is quite useful
regarding the operation of Redis® and is the recommendation for this topic in general.
Below, we will address topics specific to Icinga setups.

#### Redis® requires memory overcommit on Linux

On Linux, enable [memory overcommitting](https://www.kernel.org/doc/Documentation/vm/overcommit-accounting).

```shell
sysctl vm.overcommit_memory=1
```

To persist this setting across reboots, add the following line to [`sysctl.conf(5)`](https://man7.org/linux/man-pages/man5/sysctl.conf.5.html).
If your distribution uses systemd, a configuration file under `/etc/sysctl.d/` is required, as described by
[`systemd-sysctl.service(8)`](https://www.freedesktop.org/software/systemd/man/latest/systemd-sysctl.service.html) and
[`sysctl.d(5)`](https://man7.org/linux/man-pages/man5/sysctl.d.5.html).

```
vm.overcommit_memory = 1
```

#### Huge memory footprint and IO usage in large setups

For large setups, the default Redis® configuration is slightly problematic, since Redis® will try to perpetually save its state
to the filesystem, a process that gets triggered often enough to make the save process a permanent companion.
This will increase the memory usage by a factor of two and also cause a troublesome amount of IO.

As an improvement, Redis® can be [configured to produce the dump less often or not at all with the `save` setting](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence) in the
configuration. Be warned here, that in case of a crash (of Redis® or the whole machine) all the data after the last dump
is lost, meaning that all the events which were not already persisted by Icinga DB or persisted by Redis® are lost then.

To completely disable retention, the `save` setting must be given an empty argument:

```
save ""
```

Alternatively, to reduce the occurrences of dumps, something similar to

```
save 3600 1 900 100000
```

can be used.
In this example, a dump is performed every hour (3600s) if at least on changes occurred in that time frame
and every fifteen minutes (900s) if at least 100,000 changes occurred.

#### Redis® Access Control List

When using a shared Redis® server between Icinga DB and other applications, configuring the
[Redis® Access Control List (ACL)](https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/)
should be considered.
Creating dedicated Redis® users and ACL entries ensure that each application can only access its data.

Icinga DB only needs to access Redis® keys in the `icinga` and `icingadb` namespaces.

Using the [`ACL SETUSER`](https://redis.io/docs/latest/commands/acl-setuser/) command,
a new `icingadb` user only permitted to access its keys can be created.
Please change the password behind `>` in the following example.

```
> ACL SETUSER icingadb on >PASSWORD_CHANGE_ME ~icinga:* ~icingadb:* +@all
 OK
```

Afterward, Icinga DB needs to connect using this username and password.
This requires a change to
[Icinga 2's `IcingaDB` object](https://icinga.com/docs/icinga-2/latest/doc/09-object-types/#icingadb),
[Icinga DB's Redis® configuration](03-Configuration.md#redis-configuration) and
[Icinga DB Web's Redis® configuration](https://icinga.com/docs/icinga-db-web/latest/doc/03-Configuration/#redis-configuration).
