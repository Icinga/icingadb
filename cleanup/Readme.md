# Icinga DB History Cleanup

![Icinga Logo](https://icinga.com/wp-content/uploads/2014/06/icinga_logo.png)

#### Table of Contents

- [About](#about)
- [Cleanup.ini](#cleanup.ini)
- [Example of cleanup.ini](#example of cleanup.ini)
- [Changes in main.go](#main.go changes)

## About

This package cleans up history tables in Icinga DB periodically.

## Cleanup.ini

This file contains the table names which are required to be cleaned up under the [cleanup] section and also the 
corresponding timestamp below which the records are removed from the subsequent table. The history tables referenced in 
the ini file are:

    i. acknowledgement_history
    ii. comment_history
    iii. downtime_history
    iv. flapping_history
    v. notification_history
    vi. state_history

The tables history and user_notification_history are child tables. Hence, deleting records from their parent tables 
automatically delete the subsequent records in these tables as well.

The timestamp should be of type Duration from time package (time.Duration). 

## Example of cleanup.ini
Example of cleanup.ini is as shown below. For instance, if we want to delete all the records which are 10 days old from
current time, then the timestamp could be 10*24=2400h. Here the timestamp is same for every table. Each table can have 
different timestamps but it has to be of type Duration from time package (time.Duration):

```
[cleanup]
acknowledgement_history = "2400h"
comment_history = "2400h"
downtime_history = "2400h"
flapping_history = "2400h"
notification_history = "2400h"
state_history = "2400h"
```
