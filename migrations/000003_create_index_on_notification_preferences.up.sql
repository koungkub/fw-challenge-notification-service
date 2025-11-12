CREATE INDEX idx_notification_prefs_provider_deleted_active
ON notification_preferences (provider_type, priority)
WHERE deleted_at IS NULL;
