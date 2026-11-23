-- PAY-241 S2 stage approvals. Standalone tables: no foreign keys; relations
-- and cleanup are resolved in application transactions. Every index is built
-- by its own CONCURRENTLY migration (806-818); 819 verifies readiness.
CREATE TABLE project_stage_policy (
 workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 contract text NOT NULL CHECK (contract = 'collaboration-stage-v1'),
 enabled boolean NOT NULL,
 reason text NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 1 AND 1000),
 created_by uuid NOT NULL,
 request_id text NOT NULL CHECK (length(request_id) BETWEEN 1 AND 128),
 request_digest text NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE collaboration_stage_task (
 issue_id uuid NOT NULL, workspace_id uuid NOT NULL,
 stage text NOT NULL DEFAULT 'development' CHECK (stage IN ('development','code_review','qa','release','verification')),
 stage_revision bigint NOT NULL DEFAULT 1 CHECK (stage_revision > 0),
 role_revision bigint NOT NULL DEFAULT 0 CHECK (role_revision >= 0),
 scope_revision bigint NOT NULL DEFAULT 0 CHECK (scope_revision >= 0),
 scope_decision_id uuid,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((scope_revision = 0) = (scope_decision_id IS NULL))
);
CREATE TABLE collaboration_stage_role (
 issue_id uuid NOT NULL, workspace_id uuid NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 business_owner_user_id uuid NOT NULL, implementer_user_id uuid NOT NULL,
 product_approver_user_id uuid NOT NULL, code_reviewer_user_id uuid NOT NULL,
 qa_approver_user_id uuid NOT NULL, release_approver_user_id uuid NOT NULL,
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
 configured_by uuid NOT NULL, operation_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK (implementer_user_id <> code_reviewer_user_id),
 CHECK (qa_approver_user_id <> release_approver_user_id)
);
CREATE TABLE collaboration_stage_candidate (
 candidate_id uuid NOT NULL, issue_id uuid NOT NULL, workspace_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind = 'proposal'),
 proposal_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 parent_candidate_id uuid,
 content jsonb NOT NULL,
 content_digest text NOT NULL CHECK (content_digest ~ '^[0-9a-f]{64}$'),
 source jsonb NOT NULL,
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
 submitted_by uuid NOT NULL, operation_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((version = 1) = (parent_candidate_id IS NULL))
);
CREATE TABLE collaboration_stage_decision (
 decision_id uuid NOT NULL, issue_id uuid NOT NULL, workspace_id uuid NOT NULL,
 step text NOT NULL CHECK (step IN ('proposal','scope')),
 decision text NOT NULL CHECK (decision IN ('accepted','rejected','confirmed')),
 proposal_id uuid, candidate_id uuid, candidate_version bigint,
 content_digest text CHECK (content_digest IS NULL OR content_digest ~ '^[0-9a-f]{64}$'),
 role text NOT NULL CHECK (role = 'product_approver'),
 role_revision bigint NOT NULL CHECK (role_revision > 0),
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
 actor_user_id uuid NOT NULL,
 participant_epoch bigint NOT NULL CHECK (participant_epoch > 0),
 account_epoch bigint NOT NULL CHECK (account_epoch > 0),
 credential_kind text NOT NULL CHECK (credential_kind IN ('jwt','pat')),
 reason text NOT NULL CHECK (char_length(reason) <= 2000),
 payload jsonb NOT NULL,
 operation_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CONSTRAINT collaboration_stage_decision_shape CHECK (
  (step = 'proposal' AND decision IN ('accepted','rejected') AND proposal_id IS NOT NULL
   AND candidate_id IS NOT NULL AND candidate_version IS NOT NULL AND candidate_version > 0 AND content_digest IS NOT NULL)
  OR (step = 'scope' AND decision = 'confirmed' AND proposal_id IS NULL
   AND candidate_id IS NULL AND candidate_version IS NULL AND content_digest IS NULL))
);
CREATE TABLE collaboration_stage_child (
 child_issue_id uuid NOT NULL, root_issue_id uuid NOT NULL, workspace_id uuid NOT NULL,
 origin text NOT NULL CHECK (origin IN ('materialized','confirmed')),
 proposal_id uuid, candidate_id uuid, ordinal integer,
 decision_id uuid NOT NULL,
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
 scope_decision_id uuid,
 semantic_version bigint NOT NULL DEFAULT 1 CHECK (semantic_version > 0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK (child_issue_id <> root_issue_id),
 CONSTRAINT collaboration_stage_child_shape CHECK (
  (origin = 'materialized' AND proposal_id IS NOT NULL AND candidate_id IS NOT NULL AND ordinal BETWEEN 1 AND 20)
  OR (origin = 'confirmed' AND proposal_id IS NULL AND candidate_id IS NULL AND ordinal IS NULL))
);
-- Revisions, candidates and decisions are facts. A correction is a new row.
CREATE FUNCTION collaboration_stage_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'collaboration stage fact % is immutable', TG_TABLE_NAME USING ERRCODE = '23514';
END; $$;
CREATE TRIGGER project_stage_policy_immutable BEFORE UPDATE ON project_stage_policy
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
CREATE TRIGGER collaboration_stage_role_immutable BEFORE UPDATE ON collaboration_stage_role
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
CREATE TRIGGER collaboration_stage_candidate_immutable BEFORE UPDATE ON collaboration_stage_candidate
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
CREATE TRIGGER collaboration_stage_decision_immutable BEFORE UPDATE ON collaboration_stage_decision
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
