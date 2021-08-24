package main

var notificationCacheSchema = []string{
	`CREATE TABLE IF NOT EXISTS previous_hard_state (
	notification_id INT PRIMARY KEY,
	previous_hard_state INT NOT NULL
)`,
	`CREATE TABLE IF NOT EXISTS next_hard_state (
	object_id INT PRIMARY KEY,
	next_hard_state INT NOT NULL
)`,
	`CREATE TABLE IF NOT EXISTS next_ids (
	object_id INT NOT NULL,
	notification_id INT NOT NULL
)`,
	"CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids(object_id)",
	"CREATE INDEX IF NOT EXISTS next_ids_notification_id ON next_ids(notification_id)",
}
