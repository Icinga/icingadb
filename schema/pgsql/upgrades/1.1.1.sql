ALTER TABLE notification ALTER COLUMN name TYPE varchar(767);
COMMENT ON COLUMN notification.name IS '255+1+255+1+255, i.e. "host.name!service.name!notification.name"';
