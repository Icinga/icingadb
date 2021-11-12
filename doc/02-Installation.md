# Installation <a id="installation"></a>

## Requirements <a id="installation-requirements"></a>

* Local Redis instance (Will be installed during this documentation)
* MySQL/MariaDB server with `icingadb` database, user and schema imports (Will be installed during this documentation)

## Setting up Icinga DB <a id="setting-up-icingadb"></a>

### Package Repositories <a id="package-repositories"></a>

In order to install the latest release candidate, you have to add our `testing` repository as shown below.
We assume that you have our `release` repository already activated.
The following commands must be executed with root permissions unless noted otherwise.

#### RHEL/CentOS/Fedora Repositories <a id="package-repositories-rhel-centos"></a>

Make sure you have wget installed.

```
rpm --import https://packages.icinga.com/icinga.key

wget https://packages.icinga.com/epel/ICINGA-testing.repo -O /etc/yum.repos.d/ICINGA-testing.repo
```

#### SLES/OpenSUSE Repositories <a id="package-repositories-sles-opensuse"></a>

```
rpm --import https://packages.icinga.com/icinga.key

zypper ar https://packages.icinga.com/SUSE/ICINGA-testing.repo
zypper ref
```

#### Debian/Ubuntu Repositories <a id="package-repositories-debian-ubuntu"></a>

Debian:

```
apt-get update
apt-get -y install apt-transport-https wget gnupg

wget -O - https://packages.icinga.com/icinga.key | apt-key add -

DIST=$(awk -F"[)(]+" '/VERSION=/ {print $2}' /etc/os-release); \
 echo "deb https://packages.icinga.com/debian icinga-${DIST}-testing main" > \
 /etc/apt/sources.list.d/${DIST}-icinga-testing.list
 echo "deb-src https://packages.icinga.com/debian icinga-${DIST}-testing main" >> \
 /etc/apt/sources.list.d/${DIST}-icinga-testing.list

apt-get update
```

Ubuntu:

```
apt-get update
apt-get -y install apt-transport-https wget gnupg

wget -O - https://packages.icinga.com/icinga.key | apt-key add -

. /etc/os-release; if [ ! -z ${UBUNTU_CODENAME+x} ]; then DIST="${UBUNTU_CODENAME}"; else DIST="$(lsb_release -c| awk '{print $2}')"; fi; \
 echo "deb https://packages.icinga.com/ubuntu icinga-${DIST}-testing main" > \
 /etc/apt/sources.list.d/${DIST}-icinga-testing.list
 echo "deb-src https://packages.icinga.com/ubuntu icinga-${DIST}-testing main" >> \
 /etc/apt/sources.list.d/${DIST}-icinga-testing.list

apt-get update
```


### Installing Icinga DB <a id="installing-icingadb"></a>

RHEL/CentOS 8/Fedora:

```
dnf install icingadb
systemctl enable icingadb
systemctl start icingadb
```

RHEL/CentOS 7:

```
yum install icingadb
systemctl enable icingadb
systemctl start icingadb
```

SUSE:

```
zypper install icingadb
```

Debian/Ubuntu:

```
apt-get install icingadb
```

### Installing Icinga DB Redis <a id="configuring-icingadb-mysql"></a>

RHEL/CentOS 8/Fedora:

```
dnf install icingadb-redis

systemctl enable icingadb-redis
systemctl start icingadb-redis
```

RHEL/CentOS 7:

```
yum install icingadb-redis

systemctl enable icingadb-redis
systemctl start icingadb-redis
```

SUSE:

```
zypper install icingadb-redis

systemctl enable icingadb-redis
systemctl start icingadb-redis
```

Debian/Ubuntu:

```
apt-get install icingadb-redis
```

### Configuring MySQL <a id="configuring-icingadb-mysql"></a>

#### Installing MySQL/MariaDB database server <a id="installing-database-mysql-server"></a>

RHEL/CentOS 8/Fedora:

```
dnf install mariadb-server mariadb

systemctl enable mariadb
systemctl start mariadb

mysql_secure_installation
```

RHEL/CentOS 7:

```
yum install mariadb-server mariadb

systemctl enable mariadb
systemctl start mariadb

mysql_secure_installation
```

SUSE:

```
zypper install mysql mysql-client

systemctl enable mariadb
systemctl start mariadb
```

Debian/Ubuntu:

```
apt-get install mysql-server mysql-client

mysql_secure_installation
```

#### Setting up the MySQL database <a id="setting-up-mysql-db"></a>

Note that if you're using a version of MySQL < 5.7 or MariaDB < 10.2, the following server options must be set:

```
innodb_file_format=barracuda	
innodb_file_per_table=1	
innodb_large_prefix=1
```

Set up a MySQL database for Icinga DB:

```
# mysql -u root -p

CREATE DATABASE icingadb;
GRANT ALL ON icingadb.* TO 'icingadb'@'127.0.0.1' IDENTIFIED BY 'icingadb';
FLUSH PRIVILEGES;

quit
```

After creating the database you can import the Icinga DB schema using the
following command. Enter the root password into the prompt when asked.

```
cat /usr/share/icingadb/schema/mysql/schema.sql | mysql -u root icingadb -p
```

### Running Icinga DB <a id="running-icingadb"></a>

Foreground:

```
icingadb -config /etc/icingadb/icingadb.yml
```

Systemd service:

```
systemctl enable icingadb
systemctl start icingadb
```

### Enable remote Redis connections <a id="remote-redis"></a>

By default `icingadb-redis` listens only on `127.0.0.1`. If you want to change that (e.g. Icinga Web 2), just change `bind 127.0.0.1 ::1` and `protected-mode yes` in `/etc/icinga-redis/icingadb-redis.conf` to the interface you want to use and `protected-mode no`.

> WARNING: Make sure your host is secured by some kind of firewall, if you open Redis to an external interface. Redis, by default, does not have any authentication that prevent others from accessing it.

### Icinga DB Web

Consult the [Icinga DB Web documentation](https://icinga.com/docs/icingadb/latest/icingadb-web/doc/02-Installation/) on how to connect Icinga Web 2 with Icinga DB.
