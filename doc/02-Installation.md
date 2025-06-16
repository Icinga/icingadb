<!-- {% if index %} -->
# Installing Icinga DB

The recommended way to install Icinga DB is to use prebuilt packages for
all supported platforms from our official release repository.
Please follow the steps listed for your target operating system,
which guide you through setting up the repository and installing Icinga DB.

To upgrade an existing Icinga DB installation to a newer version,
see the [Upgrading](04-Upgrading.md) documentation for the necessary steps.

![Icinga DB Daemon](images/icingadb-daemon.png)

Before installing Icinga DB, make sure you have installed [Icinga 2](https://icinga.com/docs/icinga-2),
set up a Redis® server, and enabled the `icingadb` feature.
The Icinga 2 installation documentation covers all the necessary steps.
Additionally, Icinga offers the `icingadb-redis` package for all supported operating systems,
which ships an up-to-date Redis® open source server version and is pre-configured for the Icinga DB components.

!!! tip

    Although Icinga DB can run anywhere in an Icinga environment,
    we recommend to install it where the corresponding Icinga 2 node and Redis® server is running to
    keep latency between the components low.

<!-- {% elif not icingaDocs %} -->
## Installing the Package

If the [repository](https://packages.icinga.com) is not configured yet, please add it first.
Then use your distribution's package manager to install the `icingadb` package
or install [from source](02-Installation.md.d/From-Source.md).
<!-- {% else %} -->

<!-- {% if amazon_linux or fedora or rhel or opensuse or sles%} -->
!!! tip

    If you have SELinux enabled, the package `icingadb-selinux` is also required.
<!-- {% endif %} -->

## Setting up the Database

A MySQL (≥8.0), MariaDB (≥10.2.2), or PostgreSQL (≥9.6) database is required to run Icinga DB.
Please follow the steps listed for your target database,
which guide you through setting up the database and user and importing the schema.

![Icinga DB Database](images/icingadb-database.png)

!!! info

    In high availability setups, all Icinga DB instances must write to the same database.

### Setting up a MySQL or MariaDB Database

Set up a MySQL database for Icinga DB:

```
# mysql -u root -p

CREATE DATABASE icingadb;
CREATE USER 'icingadb'@'localhost' IDENTIFIED BY 'CHANGEME';
GRANT ALL ON icingadb.* TO 'icingadb'@'localhost';
```

After creating the database, import the Icinga DB schema using the following command:

```
mysql -u root -p icingadb </usr/share/icingadb/schema/mysql/schema.sql
```

### Setting up a PostgreSQL Database

This section walks you through configuring PostgreSQL to work with Icinga DB.

Icinga DB requires the `citext` extension which is provided by the `postgresql-contrib` package on certain operating systems.
Please check if it is available for your OS and if so, install it.

Allow authenticated sessions for the `icingadb` database user from anywhere by modifying the `pg_hba.conf` file.
The location of this file is operating system specific, but can be queried.

```
su postgres -c "psql -c 'show hba_file;'"
```

Edit `pg_hba.conf`, insert the following before everything else:

```
local all icingadb           md5
host  all icingadb 0.0.0.0/0 md5
host  all icingadb      ::/0 md5
```

If using PostgreSQL version 10 or later, you may want to use `scram-sha-256` as the `auth-method` instead of `md5`.
Also, if requests are only to be expected from certain subnets, please tighten the network masks given above.

The database uses the `en_US.UTF-8` locale, which requires an appropriate locale on your system.
Please verify with `locale -a` that something like `en_US.UTF-8` or `en_US.utf8` exists.
If not, please add it using your operating system specific method.

To apply all these changes, restart PostgreSQL.

```
systemctl restart postgresql
```

Now proceed with actually creating both user and database.

```
# su -l postgres

createuser -P icingadb
createdb -E UTF8 --locale en_US.UTF-8 -T template0 -O icingadb icingadb
psql -c 'CREATE EXTENSION IF NOT EXISTS citext;' icingadb
```

After creating the database, import the Icinga DB schema using the following command:

```
psql -U icingadb icingadb < /usr/share/icingadb/schema/pgsql/schema.sql
```

## Configuring Icinga DB

<!-- {% if from_source %} -->
Create a local `config.yml` file using [the sample configuration](../config.example.yml).
<!-- {% else %} -->
The Icinga DB package installs its configuration file to `/etc/icingadb/config.yml`.
<!-- {% endif %} -->
Most of the settings are pre-populated for a local setup.
Before running Icinga DB, adjust the Redis® and database credentials and, if necessary, the connection configuration.
The configuration file explains general settings.
All available settings can be found under [Configuration](03-Configuration.md).

## Running Icinga DB

<!-- {% if from_source %} -->
You can execute `icingadb` by running it with the locally accessible `config.yml` file:

```bash
icingadb -config /path/to/config.yml
```
<!-- {% else %} -->
The `icingadb` package automatically installs the necessary systemd unit files to run Icinga DB.
Please run the following command to enable and start its service:

```bash
systemctl enable --now icingadb
```
<!-- {% endif %} -->

## Installing Icinga DB Web

With Icinga 2, Redis®, Icinga DB and the database fully set up, it is now time to install Icinga DB Web,
which connects to both Redis® and the database to display and work with the monitoring data.

![Icinga DB Web](images/icingadb-web.png)

You have completed the instructions here and can proceed to
<!-- {% if amazon_linux %} -->
[installing Icinga DB Web on Amazon Linux](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/Amazon-Linux/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if debian %} -->
[installing Icinga DB Web on Debian](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/Debian/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if fedora %} -->
[installing Icinga DB Web on Fedora](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/Fedora/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if rhel %} -->
[installing Icinga DB Web on RHEL](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/RHEL/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if raspberry_pi_os %} -->
[installing Icinga DB Web on Raspberry Pi Os](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/Raspberry-Pi-OS/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if sles %} -->
[installing Icinga DB Web on SLES](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/SLES/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if ubuntu %} -->
[installing Icinga DB Web on Ubuntu](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/Ubuntu/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if opensuse %} -->
[installing Icinga DB Web on openSUSE](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/openSUSE/#installing-icinga-db-web-package),
<!-- {% endif %} -->
<!-- {% if from_source %} -->
[installing Icinga DB Web From Source](https://icinga.com/docs/icinga-db-web/latest/doc/02-Installation/From-Source),
<!-- {% endif %} -->
which will also guide you through the setup of the Icinga Web PHP framework,
which is required to run the Icinga DB web module.
Below is a preview of how the interface visualizes monitoring data and also supports dark and light mode:

![Icinga DB Web](images/icingadb-dashboard.png)
<!-- {% endif %} -->
