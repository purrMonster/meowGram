-- Access revisions belong to the membership transaction, including human-FK
-- cascade deletion. Installing this trigger does not rewrite any existing row.
-- API membership edits are INSERT/DELETE, never identity-changing UPDATEs.
-- migration:step
CREATE OR REPLACE FUNCTION conversation_membership_revision() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    member conversation_members%ROWTYPE;
    human_count bigint;
BEGIN
    IF TG_OP = 'DELETE' THEN member := OLD; ELSE member := NEW; END IF;
    PERFORM id FROM conversations WHERE id = member.conversation_id FOR UPDATE;
    IF member.member_kind = 'human' THEN
        UPDATE users SET membership_version = membership_version + 1
            WHERE authelia_sub = member.human_sub;
        SELECT count(*) INTO human_count FROM conversation_members
            WHERE conversation_id = member.conversation_id AND member_kind = 'human';
        -- First human restores access; last human removes access for retained bots.
        IF (TG_OP = 'INSERT' AND human_count = 1) OR (TG_OP = 'DELETE' AND human_count = 0) THEN
            UPDATE bot_principals SET membership_version = membership_version + 1
                WHERE id IN (SELECT bot_id FROM conversation_members
                    WHERE conversation_id = member.conversation_id AND member_kind = 'bot');
        END IF;
    ELSE
        UPDATE bot_principals SET membership_version = membership_version + 1
            WHERE id = member.bot_id;
    END IF;
    RETURN NULL;
END $$;

-- migration:step
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid = 'conversation_members'::regclass
                   AND tgname = 'conversation_membership_revision_trigger' AND NOT tgisinternal) THEN
        CREATE TRIGGER conversation_membership_revision_trigger
            AFTER INSERT OR DELETE ON conversation_members
            FOR EACH ROW EXECUTE FUNCTION conversation_membership_revision();
    END IF;
END $$;
