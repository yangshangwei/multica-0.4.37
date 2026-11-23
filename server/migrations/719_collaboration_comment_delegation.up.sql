BEGIN;
ALTER TABLE collaboration_delegation ADD COLUMN source_comment_id uuid,
    ADD COLUMN member_agent_id uuid, ADD COLUMN run_id uuid,
    ADD CONSTRAINT collaboration_delegation_comment_identity CHECK (
        (source_comment_id IS NULL AND member_agent_id IS NULL AND run_id IS NULL) OR
        (source_comment_id IS NOT NULL AND member_agent_id IS NOT NULL AND run_id IS NOT NULL));
CREATE FUNCTION collaboration_preserve_comment_delegation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.source_comment_id,NEW.member_agent_id,NEW.run_id)
        IS DISTINCT FROM ROW(OLD.source_comment_id,OLD.member_agent_id,OLD.run_id) THEN
        RAISE EXCEPTION 'collaboration comment delegation identity is immutable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_comment_delegation_identity BEFORE UPDATE ON collaboration_delegation
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_comment_delegation();
COMMIT;
