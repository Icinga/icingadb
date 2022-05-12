-- Icinga DB's state_history#previous_hard_state per IDO's icinga_statehistory#statehistory_id.
CREATE TABLE IF NOT EXISTS previous_hard_state (
    history_id          INT PRIMARY KEY,
    previous_hard_state INT NOT NULL
);

-- Helper table, the current last_hard_state per icinga_statehistory#object_id.
CREATE TABLE IF NOT EXISTS next_hard_state (
    object_id       INT PRIMARY KEY,
    next_hard_state INT NOT NULL
);

-- Helper table for stashing icinga_statehistory#statehistory_id until last_hard_state changes.
CREATE TABLE IF NOT EXISTS next_ids (
    object_id  INT NOT NULL,
    history_id INT NOT NULL
);

CREATE INDEX IF NOT EXISTS next_ids_object_id ON next_ids (object_id);
CREATE INDEX IF NOT EXISTS next_ids_history_id ON next_ids (history_id);
