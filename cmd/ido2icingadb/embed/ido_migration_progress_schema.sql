CREATE TABLE IF NOT EXISTS ido_migration_progress (
    environment_id CHAR(40)    NOT NULL, -- Hex SHA1. Rationale: CHAR(40) is not RDBMS-specific
    history_type   VARCHAR(63) NOT NULL,
    last_ido_id    BIGINT      NOT NULL,

    CONSTRAINT pk_ido_migration_progress PRIMARY KEY (environment_id, history_type)
);
