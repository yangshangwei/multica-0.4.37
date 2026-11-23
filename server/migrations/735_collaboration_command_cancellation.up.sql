BEGIN;
LOCK TABLE collaboration_command, collaboration_event IN ACCESS EXCLUSIVE MODE NOWAIT;

ALTER TABLE collaboration_command ADD COLUMN IF NOT EXISTS cancellation_event_id uuid;

-- Only an unambiguous manual receipt proves an old command's cancellation.
-- Bulk events and nearby timestamps cannot supply command-level provenance.
WITH singleton AS (
    SELECT c.id AS command_id, min(e.id::text)::uuid AS event_id
    FROM collaboration_command c
    JOIN collaboration_session s ON s.id=c.session_id
    JOIN collaboration_event e ON e.issue_id=s.issue_id AND e.kind='command_cancelled'
        AND e.payload->>'command_id'=c.id::text
    WHERE c.state='cancelled' AND c.reason='cancelled_by_participant' AND c.cancellation_event_id IS NULL
    GROUP BY c.id HAVING count(*)=1
)
UPDATE collaboration_command c SET cancellation_event_id=e.id
FROM singleton candidate, collaboration_event e
WHERE c.id=candidate.command_id AND e.id=candidate.event_id AND e.actor_user_id IS NOT NULL
    AND e.payload->>'action'='cancel_command' AND e.payload->>'code'='command_cancelled'
    AND e.payload->>'target_id'=c.target_id::text
    AND e.payload->>'channel_id'=c.channel_id::text
    AND e.payload->>'batch_id'=c.batch_id::text
    AND e.payload->>'command_sequence'=c.seq::text;

-- The input/proof/placement trigger remains independent and unchanged.
CREATE OR REPLACE FUNCTION collaboration_preserve_command_cancellation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='UPDATE' THEN
        IF OLD.cancellation_event_id IS NOT NULL AND NEW.cancellation_event_id IS DISTINCT FROM OLD.cancellation_event_id THEN
            RAISE EXCEPTION 'collaboration cancellation event is immutable' USING ERRCODE='23514';
        END IF;
        IF OLD.state='cancelled' AND ROW(NEW.state,NEW.reason) IS DISTINCT FROM ROW(OLD.state,OLD.reason) THEN
            RAISE EXCEPTION 'collaboration cancellation state and reason are immutable' USING ERRCODE='23514';
        END IF;
    END IF;
    IF NEW.cancellation_event_id IS NOT NULL AND NEW.state<>'cancelled' THEN
        RAISE EXCEPTION 'collaboration cancellation event requires cancelled input' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_command_cancellation_identity ON collaboration_command;
CREATE TRIGGER collaboration_command_cancellation_identity BEFORE INSERT OR UPDATE ON collaboration_command
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_command_cancellation();

CREATE OR REPLACE FUNCTION collaboration_preserve_cancellation_event() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.kind='command_cancelled' AND ROW(NEW.id,NEW.issue_id,NEW.event_seq,NEW.actor_user_id,NEW.kind,NEW.payload,NEW.created_at)
        IS DISTINCT FROM ROW(OLD.id,OLD.issue_id,OLD.event_seq,OLD.actor_user_id,OLD.kind,OLD.payload,OLD.created_at) THEN
        RAISE EXCEPTION 'collaboration cancellation evidence is immutable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_cancellation_event_identity ON collaboration_event;
CREATE TRIGGER collaboration_cancellation_event_identity BEFORE UPDATE ON collaboration_event
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_cancellation_event();
COMMIT;
