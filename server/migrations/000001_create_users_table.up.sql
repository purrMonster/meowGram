-- Enable UUID generation support if not already available
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table storing identity federated from Authelia OIDC
-- Passwords, credentials, and registration are omitted per architectural mandate
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    authelia_sub VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Unique index ensuring one-to-one mapping for Authelia subject claims
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_authelia_sub ON users (authelia_sub);
