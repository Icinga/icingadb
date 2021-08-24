package main

var stateCacheSchema = []string{
	`CREATE TABLE IF NOT EXISTS previous_hard_state (
	statehistory_id INT PRIMARY KEY,
	previous_hard_state INT NOT NULL
)`,
	`CREATE TABLE IF NOT EXISTS next_hard_state (
	object_id INT PRIMARY KEY,
	next_hard_state INT NOT NULL
)`,
	`CREATE TABLE IF NOT EXISTS next_ids (
	object_id INT NOT NULL,
	statehistory_id INT NOT NULL
)`,
	"CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids(object_id)",
	"CREATE INDEX IF NOT EXISTS next_ids_statehistory_id ON next_ids(statehistory_id)",
}
