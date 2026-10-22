CREATE TABLE IF NOT EXISTS message_idempotency (
    sender_kind TEXT NOT NULL CHECK (sender_kind IN ('human', 'bot')),
    sender_id TEXT NOT NULL,
    operation_key VARCHAR(255) NOT NULL,
    fingerprint BYTEA NOT NULL,
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    response_body BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (sender_kind, sender_id, operation_key)
);
CREATE INDEX IF NOT EXISTS idx_message_idempotency_expiry ON message_idempotency(expires_at);
