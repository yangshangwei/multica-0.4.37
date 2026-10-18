BEGIN;
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

    -- An unread join retains its own dependency and immutable input identity.
    -- Joining never consumes content; only a continuation claim may do that.
    IF OLD.state='waiting' AND NEW.state='waiting' AND OLD.group_id IS NULL AND NEW.group_id IS NOT NULL THEN
        IF NEW.channel_id IS DISTINCT FROM OLD.channel_id OR NEW.seq<>OLD.seq OR NOT EXISTS (
            SELECT 1 FROM collaboration_execution_group g
            JOIN collaboration_batch b ON b.id=g.batch_id
            JOIN collaboration_target t ON t.id=g.target_id AND t.kind='squad'
            JOIN collaboration_group_dependency d ON d.group_id=g.id AND d.command_id=OLD.id AND d.detached_at IS NULL
            JOIN collaboration_run r ON r.group_id=g.id AND r.step_kind='continuation' AND r.progress_state='prepared'
              AND r.settled_at IS NULL AND r.forced_disposition IS NULL
            WHERE g.id=NEW.group_id AND g.session_id=NEW.session_id AND g.channel_id=NEW.channel_id AND g.batch_id=NEW.batch_id
              AND g.expansion_sealed_at IS NULL AND g.settled_at IS NULL AND g.forced_disposition IS NULL
              AND b.state='in_flight' AND b.group_id=g.id
        ) THEN RAISE EXCEPTION 'collaboration unread join requires its frozen dependency and queued continuation' USING ERRCODE='23514'; END IF;
        RETURN NEW;
    END IF;
    IF OLD.state='waiting' AND OLD.group_id IS NOT NULL AND NEW.group_id IS NULL AND NEW.state IN ('waiting','cancelled') THEN
        IF NEW.channel_id IS DISTINCT FROM OLD.channel_id OR NEW.seq<>OLD.seq OR NOT EXISTS (
            SELECT 1 FROM collaboration_group_dependency d WHERE d.group_id=OLD.group_id AND d.command_id=OLD.id AND d.detached_at IS NOT NULL
        ) OR EXISTS (SELECT 1 FROM collaboration_group_input i WHERE i.group_id=OLD.group_id AND i.command_id=OLD.id)
        THEN RAISE EXCEPTION 'collaboration unread detach requires its unconsumed dependency' USING ERRCODE='23514'; END IF;
        IF NEW.state='waiting' AND NOT EXISTS (
            SELECT 1 FROM collaboration_batch b WHERE b.id=NEW.batch_id AND b.session_id=NEW.session_id
              AND b.channel_id=NEW.channel_id AND b.state='waiting' AND b.started_at IS NULL AND b.group_id IS NULL
        ) THEN RAISE EXCEPTION 'detached input requires a waiting batch' USING ERRCODE='23514'; END IF;
        RETURN NEW;
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
                    AND r.step_kind IN ('agent','leader','continuation')
                    AND r.settled_at IS NOT NULL AND r.progress_state='settled' AND r.proof_state='never_launched_proven'
                    AND r.sealed_attempt_seq IS NOT NULL AND r.sealed_attempt_seq>=r.authorized_attempt_seq
                    AND r.started_at IS NULL AND r.business_outcome IS DISTINCT FROM 'success'
                    AND (r.step_kind='continuation' OR NOT EXISTS (SELECT 1 FROM collaboration_run other WHERE other.group_id=g.id AND
                        (other.settled_at IS NULL OR other.progress_state<>'settled' OR other.proof_state<>'never_launched_proven'
                         OR other.sealed_attempt_seq IS NULL OR other.sealed_attempt_seq<other.authorized_attempt_seq
                         OR other.started_at IS NOT NULL OR other.business_outcome='success')))
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
            AND (r.parent_run_id IS NULL AND r.step_kind IN ('agent','leader') OR r.parent_run_id IS NOT NULL AND r.step_kind='continuation')
    ) THEN
        RAISE EXCEPTION 'collaboration group input requires its consuming root' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
COMMIT;
