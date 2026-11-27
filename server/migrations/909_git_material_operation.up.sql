CREATE TABLE git_material_operation (
 id uuid NOT NULL, job_id uuid NOT NULL, task_id uuid NOT NULL,
 workspace_id uuid NOT NULL, project_id uuid NOT NULL, material_id uuid NOT NULL, revision_id uuid NOT NULL,
 runtime_id uuid NOT NULL, agent_id uuid NOT NULL, authorizer_id uuid NOT NULL,
 attempt_no integer NOT NULL, action text NOT NULL, descriptor jsonb NOT NULL,
 expected_digest text NOT NULL, claim_generation bigint NOT NULL DEFAULT 0,
 challenge_hash text NOT NULL DEFAULT '', state text NOT NULL DEFAULT 'queued',
 observation_id text NOT NULL DEFAULT '', receipt jsonb, receipt_digest text NOT NULL DEFAULT '',
 permit_request_id text NOT NULL DEFAULT '', permit_expires_at timestamptz,
 blocker text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
