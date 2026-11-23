CREATE TABLE project_delivery_policy (
 workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 policy_version text NOT NULL CHECK (policy_version = 'delivery-artifact-observe-v1'),
 mode text NOT NULL CHECK (mode IN ('off', 'observe')),
 stage text NOT NULL CHECK (stage = 'done'),
 reason text NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 1 AND 1000),
 created_by uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE delivery_evidence_evaluation (
 workspace_id uuid NOT NULL, evaluator_user_id uuid NOT NULL, request_key uuid NOT NULL,
 evaluation_id uuid NOT NULL, issue_id uuid NOT NULL, project_id uuid NOT NULL,
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 request_digest text NOT NULL, request_json jsonb NOT NULL, observation jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
