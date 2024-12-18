ALTER TABLE redundancy_group_state
    ADD COLUMN is_reachable enum('n', 'y') NOT NULL DEFAULT 'y' AFTER failed;
