ALTER TABLE alert_history_ref
  DROP CONSTRAINT pk_alert_history_ref;
ALTER TABLE alert_history_ref
  ADD CONSTRAINT pk_alert_history_ref PRIMARY KEY (history_id_uuid, history_id);
