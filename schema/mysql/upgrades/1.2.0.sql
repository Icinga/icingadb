ALTER TABLE customvar_flat MODIFY COLUMN flatvalue text DEFAULT NULL;

ALTER TABLE customvar_flat
    ADD INDEX idx_customvar_flat_flatname_flatvalue (flatname, flatvalue(255)) COMMENT 'Lists filtered by custom variable';
