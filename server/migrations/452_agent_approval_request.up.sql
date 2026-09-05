-- Human approval boundary for high-risk agent actions.
--
-- The server cannot intercept a shell command running on a daemon host, so this
-- table is not a sandbox. It is the auditable record of the boundary: an agent
-- files a request describing what it intends to do, a HUMAN decides it (the
-- decision endpoint rejects machine credentials), and the agent may only record
-- execution against an approved row. A request that is never approved leaves the
-- agent with nothing to point at, which is what "Operator without approval can
-- only produce a plan" means in practice.
--
-- No foreign keys by repository rule: workspace_id / agent_id / task_id /
-- issue_id / decided_by are resolved and validated in the handler, and workspace
-- deletion cleans this table explicitly.
CREATE TABLE IF NOT EXISTS agent_approval_request (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    -- The task turn that filed the request, when one is in flight. Kept for
    -- provenance only: an approval outlives the turn that asked for it.
    task_id UUID,
    issue_id UUID,
    -- Which class of high-risk action this is. Mirrors the autonomy policy's
    -- risk classes; unknown values are rejected by the handler, so widening the
    -- product vocabulary is a code change, not a data accident.
    risk_class TEXT NOT NULL CHECK (risk_class IN (
        'production_release', 'database_migration', 'secret_access',
        'external_notification', 'destructive_operation'
    )),
    -- One-line summary of the intended action, plus the full plan the agent
    -- produced. The plan is what the human actually reviews.
    summary TEXT NOT NULL,
    plan TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'approved', 'rejected', 'executed', 'cancelled'
    )),
    decided_by UUID,
    decided_at TIMESTAMPTZ,
    decision_note TEXT NOT NULL DEFAULT '',
    executed_at TIMESTAMPTZ,
    execution_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
