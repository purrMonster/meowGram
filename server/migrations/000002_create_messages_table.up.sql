-- ==============================================================================
-- Migration: 000002_create_messages_table.up.sql
-- Epic 1.3: Real-Time WebSocket Core - Message Persistence
-- ==============================================================================

-- Ensure users.authelia_sub has an explicit UNIQUE constraint for foreign key linkage
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_users_authelia_sub'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT uq_users_authelia_sub UNIQUE (authelia_sub);
    END IF;
END $$;

-- Persistent message ledger
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id VARCHAR(255) NOT NULL REFERENCES users(authelia_sub) ON DELETE CASCADE,
    text_content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index on created_at for fast chronological timeline retrieval (recent messages first)
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages (created_at DESC);

-- Index on sender_id for user message filtering and profile history
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages (sender_id);
