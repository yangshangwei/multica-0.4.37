-- These are the exact human inputs that may be frozen by a runtime claim.
-- Reports themselves remain in the existing run-event ledger.
CREATE TABLE collaboration_test_subject (
 issue_id uuid NOT NULL,
 workspace_id uuid NOT NULL,
 command_id uuid NOT NULL,
 candidate_id uuid NOT NULL,
 candidate_version bigint NOT NULL CHECK (candidate_version > 0),
 candidate_digest text NOT NULL CHECK (candidate_digest ~ '^[0-9a-f]{64}$'),
 subject jsonb NOT NULL CHECK (jsonb_typeof(subject) = 'object'),
 bound_by_user_id uuid NOT NULL,
 participant_epoch bigint NOT NULL CHECK (participant_epoch > 0),
 account_epoch bigint NOT NULL CHECK (account_epoch > 0),
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch > 0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER collaboration_test_subject_immutable BEFORE UPDATE ON collaboration_test_subject
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
