# Icinga DB Housekeeping

![Icinga Logo](https://icinga.com/wp-content/uploads/2014/06/icinga_logo.png)

#### Table of Contents

- [About](#about)

## About

The aim of housekeeping is to clean up history tables of old entries.
The tables to be cleaned is configured under `cleanup` section in `config.yml` with duration in days. The records older than 
[current time - retention period] will be erased from the configured tables after the cleanup routine is run. 
This cleanup routine will be called every 1 hour by 
default.

## Cleanup config

The cleanup configuration can be found in [Configuration](03-Configuration.md).
