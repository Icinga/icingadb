# Icinga DB Housekeeping

![Icinga Logo](https://icinga.com/wp-content/uploads/2014/06/icinga_logo.png)

#### Table of Contents

- [About](#about)

## About

The aim of housekeeping is to clean up history tables of old entries.
The tables to be cleaned is configured under cleanup section in `config.yml` with duration. The records below 
(current time - duration) will be erased from the configured tables. This cleanup routine is called every 1 hour by 
default.

## Cleanup config

The cleanup configuration can be found in [Configuration](03-Configuration.md).
