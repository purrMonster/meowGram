-- Refuse to destroy replay protection for delivered messages.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM message_receipts) THEN
        RAISE EXCEPTION 'message receipts exist; preserve them before downgrade';
    END IF;
END $$;
DROP TABLE message_receipts;
