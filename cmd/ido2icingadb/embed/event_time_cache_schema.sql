-- Icinga DB's flapping_history#start_time per flapping_end row (IDO's icinga_flappinghistory#flappinghistory_id).
CREATE TABLE IF NOT EXISTS end_start_time (
    history_id      INT PRIMARY KEY,
    event_time      INT NOT NULL,
    event_time_usec INT NOT NULL
);

-- Helper table, the last start_time per icinga_statehistory#object_id.
CREATE TABLE IF NOT EXISTS last_start_time (
    object_id       INT PRIMARY KEY,
    event_time      INT NOT NULL,
    event_time_usec INT NOT NULL
);
