-- Migration 000002 added a UNIQUE constraint that already creates an index.
-- Remove the standalone index created by migration 000001; the constraint
-- continues enforcing uniqueness for users.authelia_sub.
BEGIN;
-- PostgreSQL bound this foreign key to the older standalone index. Recreate
-- the constraint atomically so it binds to uq_users_authelia_sub instead.
-- No rows are deleted or rewritten, and the foreign key is validated again.
ALTER TABLE messages DROP CONSTRAINT messages_sender_id_fkey;
DROP INDEX IF EXISTS idx_users_authelia_sub;
ALTER TABLE messages ADD CONSTRAINT messages_sender_id_fkey
    FOREIGN KEY (sender_id) REFERENCES users(authelia_sub) ON DELETE CASCADE;
COMMIT;
