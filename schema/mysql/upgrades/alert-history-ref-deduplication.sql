ALTER TABLE alert_history_ref DROP PRIMARY KEY;
ALTER TABLE alert_history_ref
  ADD PRIMARY KEY (history_id_uuid, history_id);
