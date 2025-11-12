CREATE TABLE IF NOT EXISTS notification_preferences (
    id BIGSERIAL PRIMARY KEY,
    provider_type notification_provider_type NOT NULL,
    provider_name TEXT NOT NULL,
    host TEXT NOT NULL,
    priority INT DEFAULT 0,
    secret_key TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
