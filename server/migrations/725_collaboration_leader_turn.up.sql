BEGIN;
LOCK TABLE collaboration_execution_group, collaboration_run IN ACCESS EXCLUSIVE MODE NOWAIT;

ALTER TABLE collaboration_execution_group ADD COLUMN next_leader_turn bigint NOT NULL DEFAULT 1
    CHECK (next_leader_turn > 0);
ALTER TABLE collaboration_run ADD COLUMN leader_turn bigint CHECK (leader_turn IS NULL OR leader_turn > 0);

-- Settled history has no recoverable wall-clock ordering requirement. Keep it
-- unchanged. Active groups need a unique causal order before new turns proceed.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM collaboration_execution_group g WHERE g.settled_at IS NULL AND (
            (SELECT count(*) FROM collaboration_run r WHERE r.group_id=g.id
                AND r.step_kind IN ('agent','leader') AND r.parent_run_id IS NULL)<>1
            OR EXISTS (SELECT 1 FROM collaboration_run r WHERE r.group_id=g.id AND (
                (r.step_kind IN ('agent','leader') AND r.parent_run_id IS NOT NULL)
                OR (r.step_kind IN ('member','continuation') AND r.parent_run_id IS NULL)
                OR (r.parent_run_id IS NOT NULL AND NOT EXISTS (
                    SELECT 1 FROM collaboration_run parent WHERE parent.id=r.parent_run_id AND parent.group_id=g.id))
            ))
        )
    ) THEN
        RAISE EXCEPTION 'active collaboration leader order requires complete run ancestry';
    END IF;
END $$;

CREATE TEMP TABLE collaboration_active_leader_order ON COMMIT DROP AS
WITH RECURSIVE ancestry AS (
    SELECT r.id AS run_id,r.group_id,r.parent_run_id,r.step_kind,ARRAY[r.id] AS path,false AS cycle
    FROM collaboration_run r JOIN collaboration_execution_group g ON g.id=r.group_id
    WHERE g.settled_at IS NULL AND r.step_kind IN ('agent','leader','continuation')
    UNION ALL
    SELECT a.run_id,a.group_id,parent.parent_run_id,parent.step_kind,a.path||parent.id,parent.id=ANY(a.path)
    FROM ancestry a JOIN collaboration_run parent ON parent.id=a.parent_run_id AND parent.group_id=a.group_id
    WHERE NOT a.cycle
)
SELECT run_id,group_id,1+count(*) FILTER (WHERE step_kind='continuation') AS leader_turn,bool_or(cycle) AS cycle
FROM ancestry GROUP BY run_id,group_id;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM collaboration_active_leader_order WHERE cycle)
        OR EXISTS (SELECT 1 FROM collaboration_active_leader_order GROUP BY group_id,leader_turn HAVING count(*)>1) THEN
        RAISE EXCEPTION 'active collaboration leader order is ambiguous; transaction timestamps cannot recover it';
    END IF;
END $$;

UPDATE collaboration_run r SET leader_turn=ordering.leader_turn
FROM collaboration_active_leader_order ordering WHERE ordering.run_id=r.id;
UPDATE collaboration_execution_group g SET next_leader_turn=ordering.last_turn+1
FROM (SELECT group_id,max(leader_turn) AS last_turn FROM collaboration_active_leader_order GROUP BY group_id) ordering
WHERE ordering.group_id=g.id;

CREATE FUNCTION collaboration_assign_leader_turn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='UPDATE' THEN
        IF NEW.leader_turn IS DISTINCT FROM OLD.leader_turn THEN
            RAISE EXCEPTION 'collaboration leader turn is immutable' USING ERRCODE='23514';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.leader_turn IS NOT NULL THEN
        RAISE EXCEPTION 'collaboration leader turn must be allocated by its group' USING ERRCODE='23514';
    END IF;
    IF NEW.step_kind IN ('agent','leader','continuation') THEN
        UPDATE collaboration_execution_group SET next_leader_turn=next_leader_turn+1
        WHERE id=NEW.group_id RETURNING next_leader_turn-1 INTO NEW.leader_turn;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'collaboration leader turn requires its group' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_run_leader_turn BEFORE INSERT OR UPDATE ON collaboration_run
    FOR EACH ROW EXECUTE FUNCTION collaboration_assign_leader_turn();
COMMIT;
