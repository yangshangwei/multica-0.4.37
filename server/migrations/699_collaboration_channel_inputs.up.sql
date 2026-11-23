BEGIN;
LOCK TABLE collaboration_batch, collaboration_channel, collaboration_command,
    collaboration_execution_group, collaboration_run, collaboration_target
    IN ACCESS EXCLUSIVE MODE NOWAIT;

ALTER TABLE collaboration_channel ADD COLUMN IF NOT EXISTS next_command_seq bigint NOT NULL DEFAULT 1
    CHECK (next_command_seq > 0);
ALTER TABLE collaboration_command ADD COLUMN IF NOT EXISTS group_id uuid;
CREATE TABLE IF NOT EXISTS collaboration_group_input (
    group_id uuid NOT NULL,
    command_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    batch_id uuid NOT NULL,
    seq bigint NOT NULL CHECK (seq > 0),
    consuming_run_id uuid NOT NULL
);

DROP TRIGGER IF EXISTS collaboration_command_identity ON collaboration_command;
DROP TRIGGER IF EXISTS collaboration_command_batch_identity ON collaboration_command;
DROP INDEX IF EXISTS idx_collaboration_command_target_sequence;
DROP INDEX IF EXISTS idx_collaboration_batch_waiting_target;
DROP INDEX IF EXISTS idx_collaboration_group_command;

DO $migration$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
        AND table_name='collaboration_target' AND column_name='next_command_seq') THEN
        -- An old cancelled non-root input has no consumption record. When its batch
        -- was also cancelled, do not guess whether cancellation preceded promotion.
        IF EXISTS (
            SELECT 1 FROM collaboration_command c
            JOIN collaboration_execution_group g ON g.batch_id=c.batch_id
            WHERE c.state='cancelled' AND g.state='cancelled' AND c.id<>g.command_id
              AND c.reason IS DISTINCT FROM 'cancelled_by_participant'
        ) THEN
            RAISE EXCEPTION 'ambiguous retained collaboration input consumption; retain the old data for reconciliation';
        END IF;
        IF EXISTS (
            SELECT 1 FROM collaboration_execution_group g
            WHERE (SELECT count(*) FROM collaboration_run r WHERE r.group_id=g.id
                AND r.parent_run_id IS NULL AND r.step_kind IN ('agent','leader')
                AND r.command_id=g.command_id AND r.channel_id=g.channel_id
                AND r.batch_id=g.batch_id AND r.session_id=g.session_id) <> 1
        ) THEN
            RAISE EXCEPTION 'retained collaboration group is missing a unique consuming root';
        END IF;
        -- Old sequences belonged to individual targets. Their cross-target enqueue
        -- order was never recorded. This deterministic linear extension preserves each
        -- target's sequence, including every real direct-agent batch's consumed order.
        -- The v4 sequence now denotes a channel position, not the old target counter.
        WITH positions AS (
            SELECT id,row_number() OVER (PARTITION BY channel_id ORDER BY seq,created_at,id) AS seq
            FROM collaboration_command
        )
        UPDATE collaboration_command c SET seq=p.seq FROM positions p WHERE p.id=c.id;
        UPDATE collaboration_channel ch SET next_command_seq=COALESCE((
            SELECT max(c.seq)+1 FROM collaboration_command c WHERE c.channel_id=ch.id
        ),1);

        UPDATE collaboration_command c SET group_id=g.id
        FROM collaboration_execution_group g
        WHERE c.batch_id=g.batch_id AND c.channel_id=g.channel_id AND c.session_id=g.session_id
            AND (c.state IN ('in_flight','succeeded','failed','stopped') OR c.id=g.command_id);
        INSERT INTO collaboration_group_input(group_id,command_id,channel_id,batch_id,seq,consuming_run_id)
        SELECT g.id,c.id,c.channel_id,c.batch_id,c.seq,r.id
        FROM collaboration_execution_group g
        JOIN collaboration_command c ON c.group_id=g.id
        JOIN collaboration_run r ON r.group_id=g.id AND r.parent_run_id IS NULL
            AND r.step_kind IN ('agent','leader') AND r.command_id=g.command_id;

    END IF;
END;
$migration$;

ALTER TABLE collaboration_target DROP COLUMN IF EXISTS next_command_seq;

-- Group/run identity guards remain unchanged. Only an unstarted command's
-- placement can move; a consumed command requires its original sealed proof.
CREATE OR REPLACE FUNCTION collaboration_preserve_command_input() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    placement_changed boolean;
    returning_input boolean;
BEGIN
    IF ROW(NEW.id,NEW.session_id,NEW.target_id,NEW.input_event_id,NEW.author_user_id,
        NEW.participant_epoch,NEW.author_auth_epoch,NEW.lifecycle_epoch,NEW.content,NEW.attachments)
        IS DISTINCT FROM ROW(OLD.id,OLD.session_id,OLD.target_id,OLD.input_event_id,OLD.author_user_id,
        OLD.participant_epoch,OLD.author_auth_epoch,OLD.lifecycle_epoch,OLD.content,OLD.attachments) THEN
        RAISE EXCEPTION 'collaboration command identity is immutable' USING ERRCODE='23514';
    END IF;

    placement_changed := ROW(NEW.channel_id,NEW.batch_id,NEW.seq) IS DISTINCT FROM
        ROW(OLD.channel_id,OLD.batch_id,OLD.seq);
    returning_input := OLD.state='in_flight' AND NEW.state IN ('waiting','cancelled')
        AND OLD.group_id IS NOT NULL AND NEW.group_id IS NULL;

    IF returning_input THEN
        IF NEW.channel_id IS DISTINCT FROM OLD.channel_id OR NEW.seq<>OLD.seq OR
            NOT EXISTS (
                SELECT 1 FROM collaboration_group_input i
                JOIN collaboration_execution_group g ON g.id=i.group_id
                JOIN collaboration_run r ON r.id=i.consuming_run_id AND r.group_id=g.id
                WHERE i.group_id=OLD.group_id AND i.command_id=OLD.id
                    AND i.channel_id=OLD.channel_id AND i.batch_id=OLD.batch_id AND i.seq=OLD.seq
                    AND g.session_id=OLD.session_id AND g.channel_id=i.channel_id AND g.batch_id=i.batch_id
                    AND g.settled_at IS NOT NULL AND g.expansion_sealed_at IS NOT NULL
                    AND r.parent_run_id IS NULL AND r.step_kind IN ('agent','leader')
                    AND r.settled_at IS NOT NULL AND r.progress_state='settled' AND r.proof_state='never_launched_proven'
                    AND r.sealed_attempt_seq IS NOT NULL AND r.sealed_attempt_seq>=r.authorized_attempt_seq
                    AND r.started_at IS NULL AND r.business_outcome IS DISTINCT FROM 'success'
                    AND NOT EXISTS (SELECT 1 FROM collaboration_run other WHERE other.group_id=g.id AND
                        (other.settled_at IS NULL OR other.progress_state<>'settled' OR other.proof_state<>'never_launched_proven'
                         OR other.sealed_attempt_seq IS NULL OR other.sealed_attempt_seq<other.authorized_attempt_seq
                         OR other.started_at IS NOT NULL OR other.business_outcome='success'))
            ) THEN
            RAISE EXCEPTION 'collaboration input return requires its sealed nonlaunch proof' USING ERRCODE='23514';
        END IF;
        IF NEW.state='waiting' AND NOT EXISTS (
            SELECT 1 FROM collaboration_session s
            JOIN issue_collaboration ic ON ic.issue_id=s.issue_id
            JOIN issue_participant p ON p.issue_id=s.issue_id AND p.user_id=NEW.author_user_id
            JOIN collaboration_account_fence f ON f.user_id=NEW.author_user_id
            JOIN "user" u ON u.id=NEW.author_user_id
            WHERE s.id=NEW.session_id AND s.queue_state<>'closed'
                AND ic.lifecycle_epoch=NEW.lifecycle_epoch AND p.epoch=NEW.participant_epoch
                AND p.revoked_at IS NULL AND f.epoch=NEW.author_auth_epoch AND u.disabled_at IS NULL
        ) THEN
            RAISE EXCEPTION 'obsolete collaboration input cannot return to waiting' USING ERRCODE='23514';
        END IF;
        IF NEW.batch_id=OLD.batch_id OR NOT EXISTS (
            SELECT 1 FROM collaboration_batch b WHERE b.id=NEW.batch_id
                AND b.session_id=NEW.session_id AND b.channel_id=NEW.channel_id
                AND b.state='waiting' AND b.started_at IS NULL AND b.group_id IS NULL
        ) THEN
            RAISE EXCEPTION 'returned collaboration input requires an unstarted batch' USING ERRCODE='23514';
        END IF;
        RETURN NEW;
    END IF;

    IF placement_changed THEN
        IF OLD.state<>'waiting' OR NEW.state<>'waiting' OR OLD.group_id IS NOT NULL OR NEW.group_id IS NOT NULL
            OR NOT EXISTS (SELECT 1 FROM collaboration_batch b WHERE b.id=OLD.batch_id
                AND b.session_id=OLD.session_id AND b.channel_id=OLD.channel_id
                AND b.state='waiting' AND b.started_at IS NULL AND b.group_id IS NULL)
            OR NOT EXISTS (SELECT 1 FROM collaboration_batch b WHERE b.id=NEW.batch_id
                AND b.session_id=NEW.session_id AND b.channel_id=NEW.channel_id
                AND b.state='waiting' AND b.started_at IS NULL AND b.group_id IS NULL) THEN
            RAISE EXCEPTION 'only unstarted collaboration inputs can change batch' USING ERRCODE='23514';
        END IF;
        IF NEW.channel_id IS DISTINCT FROM OLD.channel_id THEN
            IF NOT EXISTS (
                SELECT 1 FROM collaboration_target t
                JOIN collaboration_channel ch ON ch.id=t.channel_id AND ch.session_id=t.session_id
                WHERE t.id=NEW.target_id AND t.session_id=NEW.session_id AND t.channel_id=NEW.channel_id
                    AND t.hold_reason='switch_hold' AND NEW.seq>0 AND NEW.seq=ch.next_command_seq-1
                    AND ((t.kind='agent' AND t.resource_id=ch.root_agent_id) OR
                         (t.kind='squad' AND (SELECT count(*) FROM jsonb_array_elements(t.roster) member
                             WHERE member->>'role'='leader')=1 AND EXISTS (
                             SELECT 1 FROM jsonb_array_elements(t.roster) member
                             WHERE member->>'role'='leader' AND member->>'agent_id'=ch.root_agent_id::text)))
            ) THEN
                RAISE EXCEPTION 'collaboration input channel change requires a held target switch' USING ERRCODE='23514';
            END IF;
        ELSIF NEW.seq<>OLD.seq THEN
            RAISE EXCEPTION 'collaboration input order cannot change within a channel' USING ERRCODE='23514';
        END IF;
    END IF;

    IF NEW.group_id IS DISTINCT FROM OLD.group_id THEN
        IF OLD.group_id IS NOT NULL OR OLD.state<>'waiting' OR NEW.state<>'in_flight'
            OR NEW.group_id IS NULL OR placement_changed OR NOT EXISTS (
                SELECT 1 FROM collaboration_execution_group g
                JOIN collaboration_batch b ON b.id=g.batch_id
                WHERE g.id=NEW.group_id AND g.session_id=NEW.session_id
                    AND g.channel_id=NEW.channel_id AND g.batch_id=NEW.batch_id
                    AND g.settled_at IS NULL AND g.expansion_sealed_at IS NULL
                    AND b.state='in_flight' AND b.group_id=g.id AND b.started_at IS NOT NULL
            ) THEN
            RAISE EXCEPTION 'collaboration input consumption identity is immutable' USING ERRCODE='23514';
        END IF;
    END IF;
    IF NEW.state IS DISTINCT FROM OLD.state AND (
        (OLD.state='waiting' AND NEW.state='cancelled' AND NEW.group_id IS NULL) OR
        (OLD.state='waiting' AND NEW.state='in_flight' AND NEW.group_id IS NOT NULL) OR
        (OLD.state='in_flight' AND NEW.state IN ('succeeded','failed','stopped','cancelled') AND NEW.group_id=OLD.group_id)
    ) IS NOT TRUE THEN
        RAISE EXCEPTION 'collaboration command cannot be revived' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER collaboration_command_identity BEFORE UPDATE ON collaboration_command
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_command_input();

CREATE OR REPLACE FUNCTION collaboration_preserve_group_input() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        IF EXISTS (SELECT 1 FROM collaboration_execution_group WHERE id=OLD.group_id) THEN
            RAISE EXCEPTION 'retained collaboration group input cannot be deleted' USING ERRCODE='23514';
        END IF;
        RETURN OLD;
    END IF;
    IF TG_OP='UPDATE' THEN
        RAISE EXCEPTION 'collaboration group input identity is immutable' USING ERRCODE='23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM collaboration_command c
        JOIN collaboration_execution_group g ON g.id=NEW.group_id AND g.session_id=c.session_id
        JOIN collaboration_run r ON r.id=NEW.consuming_run_id AND r.group_id=g.id
        WHERE c.id=NEW.command_id AND c.group_id=g.id AND c.state='in_flight'
            AND c.channel_id=NEW.channel_id AND c.batch_id=NEW.batch_id AND c.seq=NEW.seq
            AND g.channel_id=NEW.channel_id AND g.batch_id=NEW.batch_id
            AND g.settled_at IS NULL AND g.expansion_sealed_at IS NULL
            AND r.session_id=g.session_id AND r.command_id=g.command_id
            AND r.channel_id=g.channel_id AND r.batch_id=g.batch_id
            AND r.parent_run_id IS NULL AND r.step_kind IN ('agent','leader')
    ) THEN
        RAISE EXCEPTION 'collaboration group input requires its consuming root' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS collaboration_group_input_identity ON collaboration_group_input;
CREATE TRIGGER collaboration_group_input_identity BEFORE INSERT OR UPDATE OR DELETE ON collaboration_group_input
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_group_input();
COMMIT;
