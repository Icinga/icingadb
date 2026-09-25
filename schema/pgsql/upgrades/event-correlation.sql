ALTER TABLE host_state ADD COLUMN acknowledgement_set_time biguint DEFAULT NULL;
ALTER TABLE service_state ADD COLUMN acknowledgement_set_time biguint DEFAULT NULL;

ALTER TABLE history ADD COLUMN alert_count biguint DEFAULT NULL;

-- alert_history_ref contains a reference to the history.id of rows that are of interest to the alert_history table.
-- When Icinga DB regularly runs its background alert history correlation job, it will use this table to find the
-- matching history.id rows based on the history_id_uuid. Once a particular incident has been closed, all the matching
-- rows pertaining to that incident will be deleted from this table.
CREATE TABLE alert_history_ref (
  history_id bytea20 NOT NULL, -- histry.id. No foreign key constraint as they might be processed out of order.
  history_id_uuid uuid NOT NULL, -- history.id. No foreign key constraint, as the ref may be written before the history row.
  host_id bytea20 NOT NULL, -- host.id
  service_id bytea20 DEFAULT NULL, -- service.id

  CONSTRAINT pk_alert_history_ref PRIMARY KEY (history_id),
  CONSTRAINT uq_alert_history_ref_history_id_uuid UNIQUE (history_id_uuid)
);

CREATE INDEX idx_alert_history_ref_host_service_id ON alert_history_ref(host_id, service_id);

ALTER TABLE alert_history_ref ALTER COLUMN history_id SET STORAGE PLAIN;
ALTER TABLE alert_history_ref ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE alert_history_ref ALTER COLUMN service_id SET STORAGE PLAIN;

CREATE TABLE alert_history (
  id bytea20 NOT NULL, -- sha1(history_id + contact_name + contactgroup_name + schedule_name + channel_name + triggered_at)
  history_id bytea20 NOT NULL, -- history.id
  environment_id bytea20 NOT NULL, -- environment.id
  contact_name citext,
  contactgroup_name citext,
  schedule_name citext,
  channel_name citext,
  triggered_at biguint NOT NULL,
  event_message text NOT NULL,

  CONSTRAINT pk_alert_history PRIMARY KEY (id),

  CONSTRAINT fk_alert_history_history FOREIGN KEY (history_id) REFERENCES history (id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_history_history_id ON alert_history(history_id);
CREATE INDEX idx_alert_history_triggered_at ON alert_history(triggered_at);

ALTER TABLE alert_history ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE alert_history ALTER COLUMN history_id SET STORAGE PLAIN;
ALTER TABLE alert_history ALTER COLUMN environment_id SET STORAGE PLAIN;
