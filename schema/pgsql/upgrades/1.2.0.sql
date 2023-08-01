ALTER TABLE customvar_flat ALTER COLUMN flatvalue DROP NOT NULL;

CREATE INDEX idx_customvar_flat_flatname_flatvalue ON customvar_flat(flatname, flatvalue);
COMMENT ON INDEX idx_customvar_flat_flatname_flatvalue IS 'Lists filtered by custom variable';
