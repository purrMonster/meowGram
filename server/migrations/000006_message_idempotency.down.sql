-- Preserve committed messages and retry responses across rollback. Older binaries
-- do not use this table; reapplying the additive up migration is safe.
SELECT 1;
