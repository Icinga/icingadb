package main

var ackCacheSchema = []string{
	`CREATE TABLE IF NOT EXISTS ack_clear_set_time (
	acknowledgement_id INT PRIMARY KEY,
	entry_time INT,
	entry_time_usec INT
)`,
	`CREATE TABLE IF NOT EXISTS last_ack_set_time (
	object_id INT PRIMARY KEY,
	entry_time INT NOT NULL,
	entry_time_usec INT NOT NULL
)`,
}
