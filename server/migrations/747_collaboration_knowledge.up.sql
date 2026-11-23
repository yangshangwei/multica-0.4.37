-- Candidate versions are source-owned retained history. Target counters and
-- invalidation routing are independent of ordinary workspace/project edits.
CREATE TABLE IF NOT EXISTS collaboration_knowledge_candidate (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    lineage_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version >= 1),
    source_workspace_id uuid NOT NULL,
    source_issue_id uuid NOT NULL,
    target_kind text NOT NULL CHECK (target_kind IN ('workspace', 'project')),
    target_workspace_id uuid NOT NULL,
    target_project_id uuid,
    content text NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 65536),
    summary text NOT NULL CHECK (octet_length(summary) BETWEEN 1 AND 2048),
    content_sha256 text NOT NULL CHECK (content_sha256 ~ '^[a-f0-9]{64}$'),
    sources jsonb NOT NULL CHECK (jsonb_typeof(sources) = 'array' AND jsonb_array_length(sources) BETWEEN 1 AND 32),
    evidence jsonb NOT NULL CHECK (jsonb_typeof(evidence) = 'array'),
    applicability jsonb NOT NULL CHECK (jsonb_typeof(applicability) = 'object'),
    proposed_by_user_id uuid NOT NULL,
    proposed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz,
    supersedes_candidate_id uuid,
    derived_from_candidate_id uuid,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'rejected', 'withdrawn', 'superseded')),
    state_revision bigint NOT NULL DEFAULT 1 CHECK (state_revision >= 1),
    approved_by_user_id uuid,
    approved_at timestamptz,
    last_decided_by_user_id uuid,
    last_decided_at timestamptz,
    last_decision_reason text CHECK (last_decision_reason IS NULL OR octet_length(last_decision_reason) BETWEEN 1 AND 1000),
    published_target_revision bigint CHECK (published_target_revision >= 1),
    CHECK ((target_kind = 'workspace' AND target_project_id IS NULL) OR (target_kind = 'project' AND target_project_id IS NOT NULL)),
    CHECK (jsonb_array_length(sources) + jsonb_array_length(evidence) <= 32),
    CHECK (content_sha256 = encode(sha256(convert_to(content, 'UTF8')), 'hex')),
    CHECK ((version = 1 AND lineage_id = id AND supersedes_candidate_id IS NULL) OR (version > 1 AND lineage_id <> id AND supersedes_candidate_id IS NOT NULL AND supersedes_candidate_id <> id)),
    CHECK (derived_from_candidate_id IS NULL OR (derived_from_candidate_id <> id AND version = 1)),
    CHECK (expires_at IS NULL OR expires_at > proposed_at),
    CHECK ((approved_at IS NULL AND approved_by_user_id IS NULL AND published_target_revision IS NULL) OR (approved_at IS NOT NULL AND approved_by_user_id IS NOT NULL AND published_target_revision IS NOT NULL)),
    CHECK (state NOT IN ('approved', 'superseded') OR approved_at IS NOT NULL),
    CHECK (state NOT IN ('pending', 'rejected') OR approved_at IS NULL),
    CHECK ((state = 'pending' AND state_revision = 1 AND last_decided_by_user_id IS NULL AND last_decided_at IS NULL AND last_decision_reason IS NULL) OR (state <> 'pending' AND last_decided_by_user_id IS NOT NULL AND last_decided_at IS NOT NULL)),
    CHECK (state NOT IN ('rejected', 'withdrawn') OR last_decision_reason IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS collaboration_knowledge_target (
    target_kind text NOT NULL CHECK (target_kind IN ('workspace', 'project')),
    target_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    knowledge_revision bigint NOT NULL DEFAULT 0 CHECK (knowledge_revision >= 0),
    CHECK (target_kind <> 'workspace' OR target_id = workspace_id)
);

-- No source/target joins are needed to replay retained jobs after deletion.
-- A null upper bound with scan_started_at set means an empty frozen scan.
CREATE TABLE IF NOT EXISTS collaboration_knowledge_invalidation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (kind IN ('scan', 'target_hint')),
    cause_id uuid NOT NULL,
    scope_kind text,
    scope_id uuid,
    scan_started_at timestamptz,
    scan_upper_bound uuid,
    scan_cursor uuid,
    target_kind text,
    target_id uuid,
    target_workspace_id uuid,
    knowledge_revision bigint,
    state text NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    last_attempted_at timestamptz,
    completed_at timestamptz,
    CHECK (
        (kind = 'scan' AND scope_kind IS NOT NULL AND scope_kind IN ('source_issue', 'source_workspace', 'all_sources')
            AND ((scope_kind = 'all_sources' AND scope_id IS NULL) OR (scope_kind <> 'all_sources' AND scope_id IS NOT NULL))
            AND target_kind IS NULL AND target_id IS NULL AND target_workspace_id IS NULL AND knowledge_revision IS NULL
            AND state IN ('pending', 'done'))
        OR (kind = 'target_hint' AND scope_kind IS NULL AND scope_id IS NULL AND scan_started_at IS NULL AND scan_upper_bound IS NULL AND scan_cursor IS NULL
            AND target_kind IS NOT NULL AND target_kind IN ('workspace', 'project') AND target_id IS NOT NULL AND target_workspace_id IS NOT NULL
            AND knowledge_revision IS NOT NULL AND knowledge_revision >= 0 AND state IN ('pending', 'delivered'))
    ),
    CHECK (scan_started_at IS NOT NULL OR (scan_upper_bound IS NULL AND scan_cursor IS NULL)),
    CHECK (scan_cursor IS NULL OR (scan_upper_bound IS NOT NULL AND scan_cursor <= scan_upper_bound)),
    CHECK (target_kind IS DISTINCT FROM 'workspace' OR target_id = target_workspace_id),
    CHECK ((state = 'pending' AND completed_at IS NULL) OR (state <> 'pending' AND completed_at IS NOT NULL))
);

ALTER TABLE agent_task_queue ADD COLUMN IF NOT EXISTS knowledge_context_version integer;
ALTER TABLE agent_task_queue ADD COLUMN IF NOT EXISTS knowledge_delivery jsonb;
ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_knowledge_metadata;
ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_knowledge_metadata CHECK (
    (knowledge_context_version IS NULL OR knowledge_context_version = 1)
    AND (knowledge_delivery IS NULL OR (knowledge_context_version IS NOT NULL AND jsonb_typeof(knowledge_delivery) = 'object' AND octet_length(knowledge_delivery::text) <= 65536))
);

CREATE OR REPLACE FUNCTION collaboration_preserve_knowledge_candidate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'knowledge candidate history must be retained' USING ERRCODE = '23514';
    END IF;
    IF ROW(NEW.id, NEW.lineage_id, NEW.version, NEW.source_workspace_id, NEW.source_issue_id, NEW.target_kind, NEW.target_workspace_id, NEW.target_project_id,
        NEW.content, NEW.summary, NEW.content_sha256, NEW.sources, NEW.evidence, NEW.applicability, NEW.proposed_by_user_id, NEW.proposed_at,
        NEW.expires_at, NEW.supersedes_candidate_id, NEW.derived_from_candidate_id)
       IS DISTINCT FROM ROW(OLD.id, OLD.lineage_id, OLD.version, OLD.source_workspace_id, OLD.source_issue_id, OLD.target_kind, OLD.target_workspace_id, OLD.target_project_id,
        OLD.content, OLD.summary, OLD.content_sha256, OLD.sources, OLD.evidence, OLD.applicability, OLD.proposed_by_user_id, OLD.proposed_at,
        OLD.expires_at, OLD.supersedes_candidate_id, OLD.derived_from_candidate_id)
       OR (OLD.approved_at IS NOT NULL AND ROW(NEW.approved_at, NEW.approved_by_user_id, NEW.published_target_revision) IS DISTINCT FROM ROW(OLD.approved_at, OLD.approved_by_user_id, OLD.published_target_revision)) THEN
        RAISE EXCEPTION 'knowledge candidate identity and approval are immutable' USING ERRCODE = '23514';
    END IF;
    IF NEW IS NOT DISTINCT FROM OLD THEN RETURN NEW; END IF;
    IF NEW.state_revision <> OLD.state_revision + 1
       OR NOT ((OLD.state = 'pending' AND NEW.state IN ('approved', 'rejected', 'withdrawn')) OR (OLD.state = 'approved' AND NEW.state IN ('withdrawn', 'superseded'))) THEN
        RAISE EXCEPTION 'invalid knowledge candidate transition' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_knowledge_candidate_immutable ON collaboration_knowledge_candidate;
CREATE TRIGGER collaboration_knowledge_candidate_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_candidate
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_candidate();

CREATE OR REPLACE FUNCTION collaboration_preserve_knowledge_invalidation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.state = 'pending' THEN
            RAISE EXCEPTION 'pending knowledge invalidation must be retained' USING ERRCODE = '23514';
        END IF;
        RETURN OLD;
    END IF;
    IF ROW(NEW.id, NEW.kind, NEW.cause_id, NEW.scope_kind, NEW.scope_id, NEW.target_kind, NEW.target_id, NEW.target_workspace_id, NEW.knowledge_revision, NEW.created_at)
        IS DISTINCT FROM ROW(OLD.id, OLD.kind, OLD.cause_id, OLD.scope_kind, OLD.scope_id, OLD.target_kind, OLD.target_id, OLD.target_workspace_id, OLD.knowledge_revision, OLD.created_at)
        OR (OLD.scan_started_at IS NOT NULL AND ROW(NEW.scan_started_at, NEW.scan_upper_bound) IS DISTINCT FROM ROW(OLD.scan_started_at, OLD.scan_upper_bound))
        OR (OLD.scan_cursor IS NOT NULL AND (NEW.scan_cursor IS NULL OR NEW.scan_cursor < OLD.scan_cursor))
        OR (OLD.state <> 'pending' AND NEW IS DISTINCT FROM OLD) THEN
        RAISE EXCEPTION 'knowledge invalidation routing is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_knowledge_invalidation_immutable ON collaboration_knowledge_invalidation;
CREATE TRIGGER collaboration_knowledge_invalidation_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_invalidation
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_invalidation();

CREATE OR REPLACE FUNCTION collaboration_preserve_knowledge_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.target_kind, NEW.target_id, NEW.workspace_id) IS DISTINCT FROM ROW(OLD.target_kind, OLD.target_id, OLD.workspace_id)
        OR (NEW.knowledge_revision <> OLD.knowledge_revision AND NEW.knowledge_revision <> OLD.knowledge_revision + 1) THEN
        RAISE EXCEPTION 'invalid knowledge target revision transition' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_knowledge_target_immutable ON collaboration_knowledge_target;
CREATE TRIGGER collaboration_knowledge_target_immutable BEFORE UPDATE ON collaboration_knowledge_target
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_target();

-- Every down file invokes this before removing even an index. Table locks
-- also refuse a first uncommitted writer instead of missing its retained data.
CREATE OR REPLACE FUNCTION collaboration_assert_knowledge_empty() RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    LOCK TABLE agent_task_queue, collaboration_knowledge_candidate, collaboration_knowledge_target, collaboration_knowledge_invalidation IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_knowledge_candidate)
        OR EXISTS (SELECT 1 FROM agent_task_queue WHERE knowledge_context_version IS NOT NULL OR knowledge_delivery IS NOT NULL)
        OR EXISTS (SELECT 1 FROM collaboration_knowledge_invalidation WHERE state = 'pending') THEN
        RAISE EXCEPTION 'refusing rollback: retained knowledge state exists' USING ERRCODE = '55000';
    END IF;
END $$;
