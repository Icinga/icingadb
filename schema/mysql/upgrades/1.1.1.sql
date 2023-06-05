ALTER TABLE notification
    MODIFY COLUMN name varchar(767) NOT NULL COMMENT '255+1+255+1+255, i.e. "host.name!service.name!notification.name"',
    MODIFY COLUMN name_ci varchar(767) COLLATE utf8mb4_unicode_ci NOT NULL;
