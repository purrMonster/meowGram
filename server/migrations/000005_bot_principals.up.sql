-- Preserve all existing OIDC subjects and message associations.
CREATE TABLE bot_principals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE bot_directory_identities (
    provider TEXT NOT NULL CHECK (provider = 'lldap'),
    directory_namespace VARCHAR(255) NOT NULL CHECK (directory_namespace <> ''),
    external_id UUID NOT NULL,
    principal_id UUID NOT NULL UNIQUE REFERENCES bot_principals(id) ON DELETE RESTRICT,
    PRIMARY KEY (provider, directory_namespace, external_id)
);
ALTER TABLE messages ADD COLUMN bot_principal_id UUID REFERENCES bot_principals(id) ON DELETE RESTRICT;
ALTER TABLE messages ALTER COLUMN sender_id DROP NOT NULL;
ALTER TABLE messages ADD CONSTRAINT messages_exactly_one_sender
    CHECK ((sender_id IS NOT NULL) <> (bot_principal_id IS NOT NULL));
CREATE INDEX idx_messages_bot_principal_id ON messages(bot_principal_id) WHERE bot_principal_id IS NOT NULL;
