-- Stage execution authorization and authenticated observations are separate
-- append-only ledgers. No record here grants a caller another dispatch.
CREATE TABLE collaboration_stage_execution_intent (
 intent_id uuid NOT NULL, workspace_id uuid NOT NULL, issue_id uuid NOT NULL,
 candidate_id uuid NOT NULL, candidate_version bigint NOT NULL CHECK (candidate_version>0),
 candidate_digest text NOT NULL CHECK (candidate_digest ~ '^[0-9a-f]{64}$'),
 lifecycle_epoch bigint NOT NULL CHECK (lifecycle_epoch>0),
 role_revision bigint NOT NULL CHECK (role_revision>0), policy_revision bigint NOT NULL CHECK (policy_revision>0),
 approval_decision_id uuid NOT NULL, request_id uuid NOT NULL,
 request_digest text NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
 consumer_subject text NOT NULL CHECK (octet_length(consumer_subject) BETWEEN 1 AND 256),
 physical_target_key text NOT NULL CHECK (physical_target_key ~ '^[0-9a-f]{64}$'),
 target_digest text NOT NULL CHECK (target_digest ~ '^[0-9a-f]{64}$'),
 artifact_digest text NOT NULL CHECK (artifact_digest ~ '^[0-9a-f]{64}$'),
 kind text NOT NULL CHECK (kind IN ('deployment','rollback')),
 invocation jsonb NOT NULL CHECK (jsonb_typeof(invocation)='object'),
 dependencies jsonb NOT NULL CHECK (jsonb_typeof(dependencies)='array'),
 admitted_at timestamptz NOT NULL, dispatch_expires_at timestamptz NOT NULL,
 CHECK (dispatch_expires_at>admitted_at AND dispatch_expires_at<=admitted_at+interval '30 seconds')
);
CREATE TABLE collaboration_stage_execution_observation (
 observation_id uuid NOT NULL, intent_id uuid NOT NULL, workspace_id uuid NOT NULL, issue_id uuid NOT NULL,
 origin text NOT NULL CHECK (origin IN ('integration','internal')),
 native_id text NOT NULL CHECK (octet_length(native_id) BETWEEN 1 AND 256),
 native_version text NOT NULL CHECK (octet_length(native_version) BETWEEN 1 AND 256),
 observation_digest text NOT NULL CHECK (observation_digest ~ '^[0-9a-f]{64}$'),
 result text NOT NULL CHECK (result IN ('unknown','not_started','succeeded','failed')),
 external_operation_id text CHECK (external_operation_id IS NULL OR octet_length(external_operation_id) BETWEEN 1 AND 256),
 observation jsonb NOT NULL CHECK (jsonb_typeof(observation)='object'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK ((result IN ('succeeded','failed') AND external_operation_id IS NOT NULL AND origin='integration')
   OR (result IN ('unknown','not_started') AND (result<>'not_started' OR external_operation_id IS NULL)))
);
CREATE TABLE collaboration_stage_rollback_observation (
 observation_id uuid NOT NULL, workspace_id uuid NOT NULL, issue_id uuid NOT NULL,
 candidate_id uuid NOT NULL, deployment_id uuid NOT NULL,
 stream_key text NOT NULL CHECK (stream_key ~ '^[0-9a-f]{64}$'),
 version bigint NOT NULL CHECK (version>0), native_version bigint NOT NULL CHECK (native_version>0),
 observation_digest text NOT NULL CHECK (observation_digest ~ '^[0-9a-f]{64}$'),
 metric jsonb NOT NULL CHECK (jsonb_typeof(metric)='object'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER collaboration_stage_execution_intent_immutable BEFORE UPDATE ON collaboration_stage_execution_intent
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
CREATE TRIGGER collaboration_stage_execution_observation_immutable BEFORE UPDATE ON collaboration_stage_execution_observation
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
CREATE TRIGGER collaboration_stage_rollback_observation_immutable BEFORE UPDATE ON collaboration_stage_rollback_observation
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();
