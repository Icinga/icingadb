CREATE TABLE IF NOT EXISTS ido_migration_progress (
    history_type VARCHAR(63) NOT NULL,
    last_ido_id  BIGINT      NOT NULL,

    CONSTRAINT pk_ido_migration_progress PRIMARY KEY (history_type)
);
