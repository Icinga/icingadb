package main

var eventTimeCacheSchema = []string{
	// Icinga DB's flapping_history#start_time per flapping_end row (IDO's icinga_flappinghistory#flappinghistory_id).
	// Similar for acknowledgements.
	`CREATE TABLE IF NOT EXISTS end_start_time (
	history_id INT PRIMARY KEY,
	event_time INT,
	event_time_usec INT
)`,
	// Helper table, the last start_time per icinga_statehistory#object_id.
	`CREATE TABLE IF NOT EXISTS last_start_time (
	object_id INT PRIMARY KEY,
	event_time INT NOT NULL,
	event_time_usec INT NOT NULL
)`,
}

var previousHardStateCacheSchema = []string{
	// Icinga DB's state_history#previous_hard_state per IDO's icinga_statehistory#statehistory_id.
	// Similar for notifications.
	`CREATE TABLE IF NOT EXISTS previous_hard_state (
	history_id INT PRIMARY KEY,
	previous_hard_state INT NOT NULL
)`,
	// Helper table, the current last_hard_state per icinga_statehistory#object_id.
	`CREATE TABLE IF NOT EXISTS next_hard_state (
	object_id INT PRIMARY KEY,
	next_hard_state INT NOT NULL
)`,
	// Helper table for stashing icinga_statehistory#statehistory_id until last_hard_state changes.
	`CREATE TABLE IF NOT EXISTS next_ids (
	object_id INT NOT NULL,
	history_id INT NOT NULL
)`,
	"CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids(object_id)",
	"CREATE INDEX IF NOT EXISTS next_ids_history_id ON next_ids(history_id)",
}
