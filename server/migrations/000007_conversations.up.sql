-- R2.1 schema only: no group writes/routes are enabled by this migration.
-- Creator kind/id is immutable provenance, deliberately not a live-principal FK:
-- a deleted human must not block deletion or erase the conversation they created.
-- Human membership/state cascades are owner-approved; conversation/bot FKs restrict.
-- Read watermark is a retained timestamp/UUID tuple, not a message FK. Deleting
-- another user's message must not block the existing R2 user-deletion cascade.
-- Membership API transactions (UST-2.1.1.3) must increment the principal counters
-- for joins/leaves and every principal whose access changes on last-human removal.
-- Constant version defaults use PostgreSQL 16's metadata-only ADD COLUMN path.
-- Step markers delimit complete DDL statements for interruption/reapply tests.

-- migration:step
CREATE TABLE IF NOT EXISTS conversations (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by_kind TEXT NOT NULL,
    created_by_id TEXT NOT NULL
);

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversations'::regclass AND conname = 'conversations_pkey') THEN
        ALTER TABLE conversations ADD CONSTRAINT conversations_pkey PRIMARY KEY (id);
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversations'::regclass AND conname = 'conversations_creator_check') THEN
        ALTER TABLE conversations ADD CONSTRAINT conversations_creator_check CHECK (created_by_kind IN ('human', 'bot') AND char_length(created_by_id) BETWEEN 1 AND 255);
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversations'::regclass AND conname = 'conversations_name_check') THEN
        ALTER TABLE conversations ADD CONSTRAINT conversations_name_check CHECK (name IS NULL OR (
                char_length(name) BETWEEN 1 AND 100 AND
                name = btrim(name, E' \t\n\r\f' || U&'\000B\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000')
            ));
    END IF;
END $$;

-- migration:step
CREATE TABLE IF NOT EXISTS conversation_members (
    conversation_id UUID NOT NULL,
    member_kind TEXT NOT NULL,
    human_sub VARCHAR(255),
    bot_id UUID,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_members'::regclass AND conname = 'conversation_members_principal_check') THEN
        ALTER TABLE conversation_members ADD CONSTRAINT conversation_members_principal_check CHECK ((member_kind = 'human' AND human_sub IS NOT NULL AND bot_id IS NULL)
                OR (member_kind = 'bot' AND bot_id IS NOT NULL AND human_sub IS NULL));
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_members'::regclass AND conname = 'conversation_members_conversation_fkey') THEN
        ALTER TABLE conversation_members ADD CONSTRAINT conversation_members_conversation_fkey FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE RESTRICT;
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_members'::regclass AND conname = 'conversation_members_human_fkey') THEN
        ALTER TABLE conversation_members ADD CONSTRAINT conversation_members_human_fkey FOREIGN KEY (human_sub) REFERENCES users(authelia_sub) ON DELETE CASCADE;
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_members'::regclass AND conname = 'conversation_members_bot_fkey') THEN
        ALTER TABLE conversation_members ADD CONSTRAINT conversation_members_bot_fkey FOREIGN KEY (bot_id) REFERENCES bot_principals(id) ON DELETE RESTRICT;
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes
                   WHERE schemaname = current_schema() AND indexname = 'idx_conversation_members_human') THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_conversation_members_human
            ON conversation_members (conversation_id, human_sub) WHERE member_kind = 'human';
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes
                   WHERE schemaname = current_schema() AND indexname = 'idx_conversation_members_bot') THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_conversation_members_bot
            ON conversation_members (conversation_id, bot_id) WHERE member_kind = 'bot';
    END IF;
END $$;

-- migration:step
CREATE TABLE IF NOT EXISTS conversation_user_state (
    user_sub VARCHAR(255) NOT NULL,
    channel_id UUID,
    muted BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMPTZ,
    read_message_id UUID
);

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_user_state'::regclass AND conname = 'conversation_user_state_key') THEN
        ALTER TABLE conversation_user_state ADD CONSTRAINT conversation_user_state_key UNIQUE NULLS NOT DISTINCT (user_sub, channel_id);
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_user_state'::regclass AND conname = 'conversation_user_state_user_fkey') THEN
        ALTER TABLE conversation_user_state ADD CONSTRAINT conversation_user_state_user_fkey FOREIGN KEY (user_sub) REFERENCES users(authelia_sub) ON DELETE CASCADE;
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_user_state'::regclass AND conname = 'conversation_user_state_conversation_fkey') THEN
        ALTER TABLE conversation_user_state ADD CONSTRAINT conversation_user_state_conversation_fkey FOREIGN KEY (channel_id) REFERENCES conversations(id) ON DELETE RESTRICT;
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'conversation_user_state'::regclass AND conname = 'conversation_user_state_read_check') THEN
        ALTER TABLE conversation_user_state ADD CONSTRAINT conversation_user_state_read_check CHECK ((read_at IS NULL) = (read_message_id IS NULL));
    END IF;
END $$;

-- migration:step
ALTER TABLE messages ADD COLUMN IF NOT EXISTS channel_id UUID;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'messages'::regclass AND conname = 'messages_channel_fkey') THEN
        ALTER TABLE messages ADD CONSTRAINT messages_channel_fkey FOREIGN KEY (channel_id) REFERENCES conversations(id) ON DELETE RESTRICT;
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes
                   WHERE schemaname = current_schema() AND indexname = 'idx_messages_channel_page') THEN
        CREATE INDEX IF NOT EXISTS idx_messages_channel_page
            ON messages (channel_id, created_at, id);
    END IF;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes
                   WHERE schemaname = current_schema() AND indexname = 'idx_messages_lounge_page') THEN
        CREATE INDEX IF NOT EXISTS idx_messages_lounge_page
            ON messages (created_at, id) WHERE channel_id IS NULL;
    END IF;
END $$;

-- migration:step
ALTER TABLE users ADD COLUMN IF NOT EXISTS membership_version BIGINT NOT NULL DEFAULT 0;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'users'::regclass AND conname = 'users_membership_version_check') THEN
        ALTER TABLE users ADD CONSTRAINT users_membership_version_check CHECK (membership_version >= 0);
    END IF;
END $$;

-- migration:step
ALTER TABLE bot_principals ADD COLUMN IF NOT EXISTS membership_version BIGINT NOT NULL DEFAULT 0;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'bot_principals'::regclass AND conname = 'bot_principals_membership_version_check') THEN
        ALTER TABLE bot_principals ADD CONSTRAINT bot_principals_membership_version_check CHECK (membership_version >= 0);
    END IF;
END $$;

