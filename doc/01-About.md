# Icinga DB

Icinga DB is a set of components for publishing, synchronizing and
visualizing monitoring data in the Icinga ecosystem, consisting of:

* The Icinga DB daemon,
  which synchronizes monitoring data between a Redis®[\*](TRADEMARKS.md#redis) server and a database
* Icinga 2 with its [Icinga DB feature](https://icinga.com/docs/icinga-2/latest/doc/14-features/#icinga-db) enabled,
  responsible for publishing the data to the Redis® server, i.e. configuration and its runtime updates, check results,
  state changes, downtimes, acknowledgements, notifications, and other events such as flapping
* And Icinga Web with the
  [Icinga DB Web](https://icinga.com/docs/icinga-db-web) module enabled,
  which connects to both Redis® and the database to display and work with the most up-to-date data

## Big Picture

![Icinga DB Architecture](images/icingadb-architecture.png)

Icinga DB consists of several components in an Icinga setup.
This section tries to help understanding how these components relate, following the architecture diagram shown above.

First things first, Icinga DB is not a database itself, but consumes and passes data from Icinga 2 to be displayed in Icinga DB Web and persisted in a relational database.

Let's start with Icinga 2.
With the Icinga DB feature enabled, Icinga 2 synchronizes its state to a Redis® server that can be queried by both the Icinga DB daemon and Icinga DB Web.

The Icinga DB daemon reads all the information from Redis®, transforms it, and finally inserts it into a relational database such as MySQL, MariaDB, or PostgreSQL.
Doing so removes load from Icinga 2 and lets Icinga DB do the more time-consuming database operations in bulk.
In addition, the Icinga DB daemon does some bookkeeping, such as removing old history entries if a retention is configured.

To display information in Icinga Web 2, the Icinga DB Web module fetches the latest service and host state information from Redis®.
Additional monitoring data and history information is retrieved from the relational database.
Icinga DB Web also connects to the Icinga 2 API with its Command Transport to acknowledge problems, trigger check executions, and so on.

These are the components of Icinga DB embedded into an Icinga setup with Icinga 2 and Icinga Web 2.

Since the Icinga DB daemon always receives the latest information from Redis®, it is an ideal candidate to distribute information further.
In addition to inserting data into a relational database, Icinga DB can also forward events to [Icinga Notifications](https://icinga.com/docs/icinga-notifications/),
as described in the [configuration section](03-Configuration.md#notifications-source-configuration).

## Installation

To install Icinga DB see [Installation](02-Installation.md).

## License

Icinga DB and the Icinga DB documentation are licensed under the terms of the
GNU General Public License Version 2.
