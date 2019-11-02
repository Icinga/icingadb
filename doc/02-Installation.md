# Installation <a id="installation"></a>

## Requirements <a id="installation-requirements"></a>

* Local Redis instance
* [Icinga 2 Core](https://icinga.com/docs/icinga2/latest/) with the `icingadb` feature enabled
* MySQL/MariaDB server with `icingadb` database, user and schema imports

Supported enterprise distributions:

* RHEL/CentOS 7
* Debian 10 Buster
* Ubuntu 18 Bionic
* SLES 15

## Setting up Icinga DB <a id="setting-up-icingadb"></a>

> TODO: More details.

### Package Repositories <a id="package-repositories"></a>

#### RHEL/CentOS Repositories <a id="package-repositories-rhel-centos"></a>

RHEL/CentOS 7:

```
yum install https://packages.icinga.com/epel/icinga-rpm-release-7-latest.noarch.rpm
```

#### SLES/OpenSUSE Repositories <a id="package-repositories-sles-opensuse"></a>

SLES 15:

```
rpm --import https://packages.icinga.com/icinga.key

zypper ar https://packages.icinga.com/SUSE/ICINGA-release.repo
zypper ref
```

#### Debian/Ubuntu Repositories <a id="package-repositories-debian-ubuntu"></a>

Debian:

```
apt-get update
apt-get -y install apt-transport-https wget gnupg

wget -O - https://packages.icinga.com/icinga.key | apt-key add -

DIST=$(awk -F"[)(]+" '/VERSION=/ {print $2}' /etc/os-release); \
 echo "deb https://packages.icinga.com/debian icinga-${DIST} main" > \
 /etc/apt/sources.list.d/${DIST}-icinga.list
 echo "deb-src https://packages.icinga.com/debian icinga-${DIST} main" >> \
 /etc/apt/sources.list.d/${DIST}-icinga.list

apt-get update
```

Ubuntu:

```
apt-get update
apt-get -y install apt-transport-https wget gnupg

wget -O - https://packages.icinga.com/icinga.key | apt-key add -

. /etc/os-release; if [ ! -z ${UBUNTU_CODENAME+x} ]; then DIST="${UBUNTU_CODENAME}"; else DIST="$(lsb_release -c| awk '{print $2}')"; fi; \
 echo "deb https://packages.icinga.com/ubuntu icinga-${DIST} main" > \
 /etc/apt/sources.list.d/${DIST}-icinga.list
 echo "deb-src https://packages.icinga.com/ubuntu icinga-${DIST} main" >> \
 /etc/apt/sources.list.d/${DIST}-icinga.list

apt-get update
```


### Installing Icinga DB <a id="installing-icingadb"></a>

> TODO: More details and packages.

RHEL/CentOS 7:

```
yum install icingadb
systemctl enable icingadb
systemctl start icingadb
```

SLES:

```
zypper install icingadb
```

Debian/Ubuntu:

```
apt-get install icingadb
```


### Configuring Icinga DB with MySQL <a id="configuring-icingadb-mysql"></a>

#### Installing MySQL/MariaDB database server <a id="installing-database-mysql-server"></a>

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
chkconfig mysqld on
service mysqld start
```

Debian/Ubuntu:

```
apt-get install mysql-server mysql-client

mysql_secure_installation
```

#### Setting up the MySQL database <a id="setting-up-mysql-db"></a>

Set up a MySQL database for Icinga DB:

```
# mysql -u root -p

CREATE DATABASE icingadb;
GRANT ALL ON icingadb.* TO 'icingadb'@'127.0.0.1' IDENTIFIED BY 'icingadb';

quit
```

After creating the database you can import the Icinga DB schema using the
following command. Enter the root password into the prompt when asked.

```
cat etc/schema/mysql/{,helper/}*.sql | mysql -uroot icingadb
```

### Running Icinga DB

Foreground:

```
groupadd -r icingadb
useradd -r -g icingadb -d / -s /sbin/nologin -c 'Icinga DB' icingadb

sudo -u icingadb ./icingadb -config icingadb.ini
```

Systemd service:

> TODO.
