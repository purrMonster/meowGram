-- ==============================================================================
-- Migration: 000002_create_messages_table.down.sql
-- Epic 1.3: Real-Time WebSocket Core - Message Persistence Rollback
-- ==============================================================================

DROP INDEX IF EXISTS idx_messages_sender_id;
DROP INDEX IF EXISTS idx_messages_created_at;
DROP TABLE IF EXISTS messages;
