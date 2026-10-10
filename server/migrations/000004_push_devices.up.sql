CREATE TABLE IF NOT EXISTS push_devices (
    id BIGSERIAL PRIMARY KEY,
    token TEXT NOT NULL UNIQUE CHECK (octet_length(token) <= 4096),
    user_sub VARCHAR(255) NOT NULL REFERENCES users(authelia_sub) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_push_devices_expiry ON push_devices (expires_at);
