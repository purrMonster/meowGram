-- Restore the standalone index if this migration is rolled back.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_authelia_sub
    ON users (authelia_sub);
