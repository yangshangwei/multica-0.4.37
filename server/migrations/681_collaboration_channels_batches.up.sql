BEGIN;

CREATE TABLE collaboration_channel (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    root_agent_id uuid NOT NULL,
    queue_state text NOT NULL DEFAULT 'paused' CHECK (queue_state IN ('paused', 'active')),
    control_version bigint NOT NULL DEFAULT 1 CHECK (control_version > 0),
    group_generation bigint NOT NULL DEFAULT 0 CHECK (group_generation >= 0),
    next_batch_seq bigint NOT NULL DEFAULT 1 CHECK (next_batch_seq > 0),
    active_group_id uuid,
    pause_reason text NOT NULL DEFAULT 'new_channel',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_batch (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    effective_target_id uuid NOT NULL,
    seq bigint NOT NULL CHECK (seq > 0),
    state text NOT NULL DEFAULT 'waiting' CHECK (state IN ('waiting', 'in_flight', 'succeeded', 'failed', 'stopped', 'cancelled')),
    root_command_id uuid,
    group_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    settled_at timestamptz,
    CHECK ((state IN ('waiting','in_flight') AND settled_at IS NULL) OR (state NOT IN ('waiting','in_flight') AND settled_at IS NOT NULL))
);

-- Reservations are retained independently of disposable runtime/task rows.
-- Only server-proven nonlaunch or authenticated sealed proof may release them.
CREATE TABLE collaboration_runtime_reservation (
    run_id uuid NOT NULL,
    execution_workspace_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    runtime_id uuid NOT NULL,
    daemon_id text NOT NULL CHECK (length(daemon_id) > 0),
    installation_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    released_at timestamptz
);

CREATE TABLE collaboration_attempt (
    run_id uuid NOT NULL,
    attempt_seq bigint NOT NULL CHECK (attempt_seq > 0),
    attempt_nonce text NOT NULL CHECK (length(attempt_nonce) BETWEEN 16 AND 128),
    claim_generation bigint NOT NULL CHECK (claim_generation > 0),
    containment text NOT NULL,
    authorized_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collaboration_business_capability (
    run_id uuid NOT NULL,
    attempt_seq bigint NOT NULL CHECK (attempt_seq > 0),
    token_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

ALTER TABLE collaboration_target ADD COLUMN channel_id uuid;
ALTER TABLE collaboration_command ADD COLUMN channel_id uuid, ADD COLUMN batch_id uuid;
ALTER TABLE collaboration_execution_group ADD COLUMN channel_id uuid, ADD COLUMN batch_id uuid;
ALTER TABLE collaboration_run ADD COLUMN channel_id uuid, ADD COLUMN batch_id uuid;

-- Preserve current v2 inputs and proof identities. A malformed existing squad
-- roster receives no guessed root; it remains ineligible until explicitly fixed.
WITH roots AS MATERIALIZED (
    SELECT t.id, t.session_id,
        CASE WHEN t.kind='agent' THEN t.resource_id ELSE (
            SELECT (r->>'agent_id')::uuid FROM jsonb_array_elements(t.roster) r
            WHERE r->>'role'='leader' AND r->>'agent_id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            LIMIT 1
        ) END AS root_agent_id
    FROM collaboration_target t
), inserted AS (
    INSERT INTO collaboration_channel (session_id, root_agent_id)
    SELECT DISTINCT session_id, root_agent_id FROM roots WHERE root_agent_id IS NOT NULL RETURNING *
)
UPDATE collaboration_target t SET channel_id=c.id FROM roots r JOIN inserted c
    ON c.session_id=r.session_id AND c.root_agent_id=r.root_agent_id WHERE t.id=r.id;

UPDATE collaboration_command c SET channel_id=t.channel_id FROM collaboration_target t WHERE t.id=c.target_id;

INSERT INTO collaboration_batch (id,session_id,channel_id,effective_target_id,seq,state,root_command_id,group_id,created_at,started_at,settled_at)
SELECT c.id,c.session_id,c.channel_id,c.target_id,
    row_number() OVER (PARTITION BY c.channel_id ORDER BY c.created_at,c.id),c.state,c.id,g.id,c.created_at,
    CASE WHEN c.state='in_flight' THEN c.created_at ELSE NULL END,
    CASE WHEN c.state='in_flight' THEN NULL ELSE COALESCE(g.settled_at,c.created_at) END
FROM collaboration_command c LEFT JOIN collaboration_execution_group g ON g.command_id=c.id
WHERE c.state<>'waiting' AND c.channel_id IS NOT NULL;
UPDATE collaboration_command SET batch_id=id WHERE state<>'waiting' AND channel_id IS NOT NULL;

WITH waiting AS (
    SELECT c.*, CASE WHEN t.kind='agent' THEN COALESCE((
        SELECT other.target_id FROM collaboration_command other JOIN collaboration_target ot ON ot.id=other.target_id
        WHERE other.channel_id=c.channel_id AND other.state='waiting' AND ot.kind='squad'
        ORDER BY other.created_at,other.id LIMIT 1
    ), c.target_id) ELSE c.target_id END AS effective_target_id
    FROM collaboration_command c JOIN collaboration_target t ON t.id=c.target_id
    WHERE c.state='waiting' AND c.channel_id IS NOT NULL
), combined AS (
    SELECT min(id::text)::uuid AS id,session_id,channel_id,effective_target_id,min(created_at) AS created_at
    FROM waiting GROUP BY session_id,channel_id,effective_target_id
)
INSERT INTO collaboration_batch (id,session_id,channel_id,effective_target_id,seq,created_at)
SELECT id,session_id,channel_id,effective_target_id,
    (SELECT count(*) FROM collaboration_batch b WHERE b.channel_id=combined.channel_id)
    + row_number() OVER (PARTITION BY channel_id ORDER BY created_at,id),created_at
FROM combined;

WITH waiting AS (
    SELECT c.id,c.channel_id,CASE WHEN t.kind='agent' THEN COALESCE((
        SELECT other.target_id FROM collaboration_command other JOIN collaboration_target ot ON ot.id=other.target_id
        WHERE other.channel_id=c.channel_id AND other.state='waiting' AND ot.kind='squad'
        ORDER BY other.created_at,other.id LIMIT 1
    ),c.target_id) ELSE c.target_id END AS effective_target_id
    FROM collaboration_command c JOIN collaboration_target t ON t.id=c.target_id WHERE c.state='waiting' AND c.channel_id IS NOT NULL
)
UPDATE collaboration_command c SET batch_id=b.id FROM waiting w JOIN collaboration_batch b
    ON b.channel_id=w.channel_id AND b.effective_target_id=w.effective_target_id AND b.state='waiting'
WHERE c.id=w.id;

UPDATE collaboration_execution_group g SET channel_id=c.channel_id,batch_id=c.batch_id FROM collaboration_command c WHERE c.id=g.command_id;
UPDATE collaboration_run r SET channel_id=g.channel_id,batch_id=g.batch_id FROM collaboration_execution_group g WHERE g.id=r.group_id;
UPDATE collaboration_channel ch SET next_batch_seq=COALESCE((SELECT max(seq)+1 FROM collaboration_batch b WHERE b.channel_id=ch.id),1),
    group_generation=COALESCE((SELECT max(generation) FROM collaboration_execution_group g WHERE g.channel_id=ch.id),0),
    active_group_id=CASE WHEN (SELECT count(*) FROM collaboration_execution_group g WHERE g.channel_id=ch.id AND settled_at IS NULL)=1
        THEN (SELECT id FROM collaboration_execution_group g WHERE g.channel_id=ch.id AND settled_at IS NULL) ELSE NULL END;

CREATE FUNCTION collaboration_preserve_batch_input() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='UPDATE' AND ROW(NEW.channel_id,NEW.batch_id) IS DISTINCT FROM ROW(OLD.channel_id,OLD.batch_id) THEN
        RAISE EXCEPTION 'collaboration input batch identity is immutable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER collaboration_command_batch_identity BEFORE UPDATE ON collaboration_command FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_batch_input();
CREATE TRIGGER collaboration_group_batch_identity BEFORE UPDATE ON collaboration_execution_group FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_batch_input();
CREATE TRIGGER collaboration_run_batch_identity BEFORE UPDATE ON collaboration_run FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_batch_input();

COMMIT;
