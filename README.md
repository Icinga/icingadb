# IMPORTANT (28.05.2021)

> :warning: **We've changed a lot to prepare for Icinga DB 1.0.0 RC2** :warning:

You will have to do the following steps to upgrade Icinga DB to the current master:
 
1. Completely stop Icinga 2 and Icinga DB
2. Flush your Redis (`redis-cli flushall`) - We made a lot of changes to our Redis schema, so this is necessary
3. Upgrade Icinga 2 to the latest snapshot/master
4. Upgrade Icinga DB to latest master
5. Upgrade the Icinga DB SQL schema (`mysql icingadb < schema/1.0.0-rc2.sql`)
6. Copy `config.yml.example` to `config.yml` and change it to your needs (The config file has changed and we don't use the old `icingadb.ini` config anymore)
7. Start Icinga 2 and Icinga DB (For Icinga DB use `go run cmd/icingadb/main.go`)

# Icinga DB

![Icinga Logo](https://icinga.com/wp-content/uploads/2014/06/icinga_logo.png)

#### Table of Contents

- [About](#about)
- [License](#license)
- [Installation](#installation)
- [Documentation](#documentation)
- [Support](#support)
- [Contributing](#contributing)

## About

Icinga DB serves as a synchronisation daemon between Icinga 2 (Redis) and Icinga Web 2 (MySQL). It synchronises configuration, state and history of an Icinga 2 environment using checksums.

Icinga DB also supports reading from multiple environments and writing into a single MySQL instance.

## License

Icinga DB and the Icinga DB documentation are licensed under the terms of the GNU
General Public License Version 2, you will find a copy of this license in the
LICENSE file included in the source package.

## Installation

For installing Icinga DB please check the [installation chapter](https://icinga.com/docs/icingadb/latest/doc/02-Installation/)
in the documentation.

## Documentation

The documentation is located in the [doc/](doc/) directory and also available
on [icinga.com/docs](https://icinga.com/docs/icingadb/latest/).

## Support

Check the [project website](https://icinga.com) for status updates. Join the
[community channels](https://icinga.com/community/) for questions
or ask an Icinga partner for [professional support](https://icinga.com/support/).

## Contributing

There are many ways to contribute to Icinga -- whether it be sending patches,
testing, reporting bugs, or reviewing and updating the documentation. Every
contribution is appreciated!

Please continue reading in the [contributing chapter](CONTRIBUTING.md).
