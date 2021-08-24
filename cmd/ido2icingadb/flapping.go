package main

var flappingCacheSchema = []string{
	`CREATE TABLE IF NOT EXISTS flapping_end_start_time (
	flappinghistory_id INT PRIMARY KEY,
	event_time INT,
	event_time_usec INT
)`,
	`CREATE TABLE IF NOT EXISTS last_flapping_start_time (
	object_id INT PRIMARY KEY,
	event_time INT NOT NULL,
	event_time_usec INT NOT NULL
)`,
}
