-- Migration 000002 added a UNIQUE constraint that already creates an index.
-- Remove the standalone index created by migration 000001; the constraint
-- continues enforcing uniqueness for users.authelia_sub.
DROP INDEX IF EXISTS idx_users_authelia_sub;
