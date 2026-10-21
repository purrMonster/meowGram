-- Intentionally retain schema and data. Older binaries cannot read bot rows;
-- rollback means disabling bot access on a compatible binary, not deleting history.
-- Destructive reversal requires separate owner authorization.
SELECT 1;
