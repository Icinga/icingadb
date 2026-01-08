ALTER TABLE user_notification_history ADD INDEX idx_user_notification_history_notification_history_id (notification_history_id) COMMENT 'Speed up ON DELETE CASCADE';
