ALTER TABLE host_state ADD COLUMN acknowledgement_set_time bigint unsigned DEFAULT NULL AFTER is_sticky_acknowledgement;
ALTER TABLE service_state ADD COLUMN acknowledgement_set_time bigint unsigned DEFAULT NULL AFTER is_sticky_acknowledgement;

ALTER TABLE history ADD COLUMN alert_count bigint unsigned DEFAULT NULL;

-- alert_history_ref contains a reference to the history.id of rows that are of interest to the alert_history table.
-- When Icinga DB regularly runs its background alert history correlation job, it will use this table to find the
-- matching history.id rows based on the history_id_uuid. Once a particular incident has been closed, all the matching
-- rows pertaining to that incident will be deleted from this table.
CREATE TABLE alert_history_ref (
  history_id binary(20) NOT NULL, -- history.id. No foreign key constraint, as the ref may be written before the history row.
  history_id_uuid binary(16) NOT NULL, -- uuid of the history.id, used for event correlation with the alert_history table.
  host_id binary(20) NOT NULL, -- host.id
  service_id binary(20) DEFAULT NULL, -- service.id

  PRIMARY KEY (history_id),
  UNIQUE INDEX uq_alert_history_ref_history_id_uuid (history_id_uuid),
  INDEX idx_alert_history_ref_host_service_id (host_id, service_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE alert_history (
  id binary(20) NOT NULL, -- sha1(history_id + contact_name + contactgroup_name + schedule_name + channel_name + triggered_at)
  history_id binary(20) NOT NULL, -- history.id
  environment_id binary(20) NOT NULL, -- environment.id,
  contact_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  contactgroup_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  schedule_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  channel_name text DEFAULT NULL COLLATE utf8mb4_unicode_ci,
  triggered_at bigint unsigned NOT NULL,
  event_message longtext NOT NULL,

  PRIMARY KEY (id),

  CONSTRAINT fk_alert_history_history FOREIGN KEY (history_id) REFERENCES history (id) ON DELETE CASCADE,

  INDEX idx_alert_history_triggered_at (triggered_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;
