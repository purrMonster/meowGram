-- Receipts are retained indefinitely so restored producers can replay safely.
CREATE TABLE message_receipts (
    sender_kind TEXT NOT NULL CHECK (sender_kind IN ('human','bot')),
    sender_id TEXT NOT NULL,
    request_key UUID NOT NULL,
    request_hash BYTEA NOT NULL,
    message_id UUID NOT NULL UNIQUE REFERENCES messages(id) ON DELETE RESTRICT,
    PRIMARY KEY (sender_kind, sender_id, request_key)
);
