CREATE TABLE collaboration_stage_submission (
 submission_id uuid NOT NULL,
 issue_id uuid NOT NULL,
 workspace_id uuid NOT NULL,
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 role_revision bigint NOT NULL CHECK (role_revision > 0),
 stage_revision bigint NOT NULL CHECK (stage_revision > 0),
 scope_decision_id uuid NOT NULL,
 candidate_id uuid NOT NULL,
 candidate_version bigint NOT NULL CHECK (candidate_version > 0),
 content_digest text NOT NULL CHECK (content_digest ~ '^[0-9a-f]{64}$'),
 release_decision_id uuid NOT NULL,
 execution_observation_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 actor_user_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER collaboration_stage_submission_immutable BEFORE UPDATE ON collaboration_stage_submission
FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
