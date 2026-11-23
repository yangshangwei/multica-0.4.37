-- An absent project override means the versioned built-in policy, not a
-- fabricated human authorization row. All relationships are application-owned.
CREATE TABLE IF NOT EXISTS project_knowledge_policy (
    project_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    is_enabled boolean NOT NULL,
    max_candidates smallint NOT NULL CHECK (max_candidates BETWEEN 1 AND 3),
    builtin_rule_version text NOT NULL CHECK (octet_length(builtin_rule_version) BETWEEN 1 AND 64),
    changed_by_user_id uuid NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS collaboration_knowledge_job (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    source_workspace_id uuid NOT NULL,
    source_issue_id uuid NOT NULL,
    project_id uuid NOT NULL,
    submission_id uuid NOT NULL,
    delivery_event_seq bigint NOT NULL CHECK (delivery_event_seq >= 1),
    accepted_event_seq bigint NOT NULL CHECK (accepted_event_seq > delivery_event_seq),
    validated_close_epoch bigint NOT NULL CHECK (validated_close_epoch >= 1),
    source_reader_user_id uuid NOT NULL,
    policy_revision bigint NOT NULL CHECK (policy_revision >= 0),
    builtin_rule_version text NOT NULL CHECK (octet_length(builtin_rule_version) BETWEEN 1 AND 64),
    max_candidates smallint NOT NULL CHECK (max_candidates BETWEEN 1 AND 3),
    phase text NOT NULL DEFAULT 'queued' CHECK (phase IN ('queued','generating','checking','settling','completed','review_pending','waiting_configuration','skipped','cancelled')),
    reason_code text CHECK (reason_code ~ '^[a-z][a-z0-9_]{0,95}$'),
    batch_manifest jsonb CHECK (jsonb_typeof(batch_manifest) = 'array' AND jsonb_array_length(batch_manifest) BETWEEN 1 AND 7 AND octet_length(batch_manifest::text) <= 16384),
    execution_revision bigint NOT NULL DEFAULT 1 CHECK (execution_revision >= 1),
    db_retry_count smallint NOT NULL DEFAULT 0 CHECK (db_retry_count BETWEEN 0 AND 3),
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    lease_token uuid,
    lease_generation bigint NOT NULL DEFAULT 0 CHECK (lease_generation >= 0),
    lease_expires_at timestamptz,
    result jsonb CHECK (jsonb_typeof(result) = 'object' AND octet_length(result::text) <= 65536),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    completed_at timestamptz,
    CHECK ((lease_token IS NULL AND lease_expires_at IS NULL) OR (lease_token IS NOT NULL AND lease_expires_at IS NOT NULL AND lease_generation >= 1))
);

-- Revalidation occupies at most six stable slots; slot six is the one new
-- candidate batch. No retry can change or truncate the predecessor manifest.
CREATE TABLE IF NOT EXISTS collaboration_knowledge_batch (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL,
    batch_index smallint NOT NULL CHECK (batch_index BETWEEN 0 AND 6),
    intent text NOT NULL CHECK (intent IN ('create','revalidate')),
    predecessor_ids uuid[] NOT NULL DEFAULT '{}',
    phase text NOT NULL DEFAULT 'queued' CHECK (phase IN ('queued','generating','checking','settling','completed','review_pending','skipped','cancelled')),
    execution_revision bigint NOT NULL DEFAULT 1 CHECK (execution_revision >= 1),
    generation_attempt_id uuid,
    check_attempt_id uuid,
    result jsonb CHECK (jsonb_typeof(result) = 'object' AND octet_length(result::text) <= 65536),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    completed_at timestamptz,
    CHECK ((intent = 'create' AND batch_index = 6 AND cardinality(predecessor_ids) = 0)
        OR (intent = 'revalidate' AND batch_index < 6 AND cardinality(predecessor_ids) BETWEEN 1 AND 3)),
    CHECK (array_position(predecessor_ids, NULL) IS NULL)
);

-- Model uncertainty is retained as its own terminal attempt. Explicit retries
-- append another attempt/execution revision; they never rewrite that history.
CREATE TABLE IF NOT EXISTS collaboration_knowledge_attempt (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL,
    batch_id uuid NOT NULL,
    execution_revision bigint NOT NULL CHECK (execution_revision >= 1),
    stage text NOT NULL CHECK (stage IN ('generating','checking')),
    attempt_number smallint NOT NULL CHECK (attempt_number BETWEEN 1 AND 3),
    lease_generation bigint NOT NULL CHECK (lease_generation >= 1),
    state text NOT NULL DEFAULT 'running' CHECK (state IN ('running','succeeded','uncertain','failed')),
    configuration_fingerprint text NOT NULL CHECK (configuration_fingerprint ~ '^[a-f0-9]{64}$'),
    requested_model text NOT NULL CHECK (octet_length(requested_model) BETWEEN 1 AND 256),
    source_snapshot_sha256 text NOT NULL CHECK (source_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
    target_snapshot_sha256 text NOT NULL CHECK (target_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
    target_revision bigint NOT NULL CHECK (target_revision >= 0),
    policy_revision bigint NOT NULL CHECK (policy_revision >= 0),
    request_sha256 text NOT NULL CHECK (request_sha256 ~ '^[a-f0-9]{64}$'),
    input_source_closure jsonb NOT NULL CHECK (jsonb_typeof(input_source_closure) = 'array' AND octet_length(input_source_closure::text) <= 2097152),
    support_dependencies jsonb NOT NULL CHECK (jsonb_typeof(support_dependencies) = 'array' AND octet_length(support_dependencies::text) <= 131072),
    response_json jsonb CHECK (jsonb_typeof(response_json) = 'object' AND octet_length(response_json::text) <= 131072),
    actual_model text CHECK (octet_length(actual_model) BETWEEN 1 AND 256),
    usage jsonb NOT NULL DEFAULT '{"status":"unknown"}' CHECK (jsonb_typeof(usage) = 'object' AND octet_length(usage::text) <= 16384),
    reason_code text CHECK (reason_code ~ '^[a-z][a-z0-9_]{0,95}$'),
    started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    finished_at timestamptz,
    CHECK ((state = 'running' AND finished_at IS NULL AND response_json IS NULL) OR (state <> 'running' AND finished_at IS NOT NULL)),
    CHECK (state <> 'succeeded' OR response_json IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS collaboration_knowledge_evaluation (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL,
    batch_id uuid NOT NULL,
    generation_attempt_id uuid NOT NULL,
    check_attempt_id uuid NOT NULL,
    candidate_id uuid NOT NULL,
    candidate_version bigint NOT NULL CHECK (candidate_version >= 1),
    candidate_content_sha256 text NOT NULL CHECK (candidate_content_sha256 ~ '^[a-f0-9]{64}$'),
    intent text NOT NULL CHECK (intent IN ('create','revalidate','revise')),
    predecessor_candidate_id uuid,
    source_workspace_id uuid NOT NULL,
    source_issue_id uuid NOT NULL,
    project_id uuid NOT NULL,
    submission_id uuid NOT NULL,
    accepted_event_seq bigint NOT NULL CHECK (accepted_event_seq >= 1),
    validated_close_epoch bigint NOT NULL CHECK (validated_close_epoch >= 1),
    policy_revision bigint NOT NULL CHECK (policy_revision >= 0),
    builtin_rule_version text NOT NULL CHECK (octet_length(builtin_rule_version) BETWEEN 1 AND 64),
    target_revision bigint NOT NULL CHECK (target_revision >= 0),
    source_snapshot_sha256 text NOT NULL CHECK (source_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
    target_snapshot_sha256 text NOT NULL CHECK (target_snapshot_sha256 ~ '^[a-f0-9]{64}$'),
    verdict text NOT NULL CHECK (verdict IN ('pass','duplicate','review_pending','transient')),
    reason_code text NOT NULL CHECK (reason_code ~ '^[a-z][a-z0-9_]{0,95}$'),
    input_source_closure jsonb NOT NULL CHECK (jsonb_typeof(input_source_closure) = 'array' AND octet_length(input_source_closure::text) <= 2097152),
    support_dependencies jsonb NOT NULL CHECK (jsonb_typeof(support_dependencies) = 'array' AND octet_length(support_dependencies::text) <= 131072),
    model_result jsonb NOT NULL CHECK (jsonb_typeof(model_result) = 'object' AND octet_length(model_result::text) <= 65536),
    usage jsonb NOT NULL CHECK (jsonb_typeof(usage) = 'object' AND octet_length(usage::text) <= 32768),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK ((intent = 'create' AND predecessor_candidate_id IS NULL)
        OR (intent = 'revalidate' AND predecessor_candidate_id IS NOT NULL AND predecessor_candidate_id = candidate_id)
        OR (intent = 'revise' AND predecessor_candidate_id IS NOT NULL AND predecessor_candidate_id <> candidate_id))
);

CREATE TABLE IF NOT EXISTS collaboration_knowledge_validity (
    candidate_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    evaluation_id uuid NOT NULL,
    source_issue_id uuid NOT NULL,
    project_id uuid NOT NULL,
    submission_id uuid NOT NULL,
    accepted_event_seq bigint NOT NULL CHECK (accepted_event_seq >= 1),
    validated_close_epoch bigint NOT NULL CHECK (validated_close_epoch >= 1),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS proposal_origin text NOT NULL DEFAULT 'human' CHECK (proposal_origin IN ('human','automation'));
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS decision_origin text NOT NULL DEFAULT 'human' CHECK (decision_origin IN ('human','automation'));
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS approval_origin text NOT NULL DEFAULT 'human' CHECK (approval_origin IN ('human','automation'));
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS automation_job_id uuid;
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS automation_policy_revision bigint CHECK (automation_policy_revision >= 0);
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS automation_rule_version text CHECK (octet_length(automation_rule_version) BETWEEN 1 AND 64);
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS approval_job_id uuid;
ALTER TABLE collaboration_knowledge_candidate ADD COLUMN IF NOT EXISTS last_decision_job_id uuid;
ALTER TABLE collaboration_knowledge_candidate ALTER COLUMN proposed_by_user_id DROP NOT NULL;
ALTER TABLE collaboration_knowledge_candidate DROP CONSTRAINT IF EXISTS collaboration_knowledge_proposal_origin;
ALTER TABLE collaboration_knowledge_candidate ADD CONSTRAINT collaboration_knowledge_proposal_origin CHECK (
    (proposal_origin = 'human' AND proposed_by_user_id IS NOT NULL AND automation_job_id IS NULL AND automation_policy_revision IS NULL AND automation_rule_version IS NULL)
    OR (proposal_origin = 'automation' AND proposed_by_user_id IS NULL AND automation_job_id IS NOT NULL AND automation_policy_revision IS NOT NULL AND automation_rule_version IS NOT NULL AND target_kind = 'project'));
-- Preserve check5: the human proposal service matches that exact expiry name.
ALTER TABLE collaboration_knowledge_candidate DROP CONSTRAINT IF EXISTS collaboration_knowledge_candidate_check6;
ALTER TABLE collaboration_knowledge_candidate ADD CONSTRAINT collaboration_knowledge_candidate_check6 CHECK (
    (approved_at IS NULL AND approved_by_user_id IS NULL AND published_target_revision IS NULL AND approval_job_id IS NULL)
    OR (approved_at IS NOT NULL AND published_target_revision IS NOT NULL AND
        ((approval_origin = 'human' AND approved_by_user_id IS NOT NULL AND approval_job_id IS NULL)
         OR (approval_origin = 'automation' AND approved_by_user_id IS NULL AND approval_job_id IS NOT NULL AND proposal_origin = 'automation'))));
ALTER TABLE collaboration_knowledge_candidate DROP CONSTRAINT IF EXISTS collaboration_knowledge_candidate_check9;
ALTER TABLE collaboration_knowledge_candidate ADD CONSTRAINT collaboration_knowledge_candidate_check9 CHECK (
    (state = 'pending' AND state_revision = 1 AND last_decided_by_user_id IS NULL AND last_decided_at IS NULL AND last_decision_reason IS NULL AND last_decision_job_id IS NULL)
    OR (state <> 'pending' AND last_decided_at IS NOT NULL AND
        ((decision_origin = 'human' AND last_decided_by_user_id IS NOT NULL AND last_decision_job_id IS NULL)
         OR (decision_origin = 'automation' AND last_decided_by_user_id IS NULL AND last_decision_job_id IS NOT NULL))));

ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_knowledge_metadata;
ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_knowledge_metadata CHECK (
    (knowledge_context_version IS NULL OR knowledge_context_version IN (1,2))
    AND (knowledge_delivery IS NULL OR (knowledge_context_version IS NOT NULL AND jsonb_typeof(knowledge_delivery) = 'object' AND octet_length(knowledge_delivery::text) <= 65536)));

ALTER TABLE collaboration_operation DROP CONSTRAINT IF EXISTS collaboration_operation_scope_kind_check;
ALTER TABLE collaboration_operation ADD CONSTRAINT collaboration_operation_scope_kind_check CHECK (scope_kind IN ('create','issue','project'));

CREATE OR REPLACE FUNCTION collaboration_preserve_knowledge_candidate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'knowledge candidate history must be retained' USING ERRCODE = '23514';
    END IF;
    IF ROW(NEW.id, NEW.lineage_id, NEW.version, NEW.source_workspace_id, NEW.source_issue_id, NEW.target_kind, NEW.target_workspace_id, NEW.target_project_id,
        NEW.content, NEW.summary, NEW.content_sha256, NEW.sources, NEW.evidence, NEW.applicability, NEW.proposed_by_user_id, NEW.proposed_at,
        NEW.expires_at, NEW.supersedes_candidate_id, NEW.derived_from_candidate_id, NEW.proposal_origin, NEW.automation_job_id, NEW.automation_policy_revision, NEW.automation_rule_version)
       IS DISTINCT FROM ROW(OLD.id, OLD.lineage_id, OLD.version, OLD.source_workspace_id, OLD.source_issue_id, OLD.target_kind, OLD.target_workspace_id, OLD.target_project_id,
        OLD.content, OLD.summary, OLD.content_sha256, OLD.sources, OLD.evidence, OLD.applicability, OLD.proposed_by_user_id, OLD.proposed_at,
        OLD.expires_at, OLD.supersedes_candidate_id, OLD.derived_from_candidate_id, OLD.proposal_origin, OLD.automation_job_id, OLD.automation_policy_revision, OLD.automation_rule_version)
       OR (OLD.approved_at IS NOT NULL AND ROW(NEW.approved_at, NEW.approved_by_user_id, NEW.published_target_revision, NEW.approval_origin, NEW.approval_job_id)
          IS DISTINCT FROM ROW(OLD.approved_at, OLD.approved_by_user_id, OLD.published_target_revision, OLD.approval_origin, OLD.approval_job_id)) THEN
        RAISE EXCEPTION 'knowledge candidate identity and approval are immutable' USING ERRCODE = '23514';
    END IF;
    IF NEW IS NOT DISTINCT FROM OLD THEN RETURN NEW; END IF;
    IF NEW.state_revision <> OLD.state_revision + 1
       OR NOT ((OLD.state = 'pending' AND NEW.state IN ('approved', 'rejected', 'withdrawn')) OR (OLD.state = 'approved' AND NEW.state IN ('withdrawn', 'superseded'))) THEN
        RAISE EXCEPTION 'invalid knowledge candidate transition' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION preserve_project_knowledge_policy() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.project_id, NEW.workspace_id) IS DISTINCT FROM ROW(OLD.project_id, OLD.workspace_id)
       OR NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'invalid knowledge policy revision' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS project_knowledge_policy_revision ON project_knowledge_policy;
CREATE TRIGGER project_knowledge_policy_revision BEFORE UPDATE ON project_knowledge_policy
    FOR EACH ROW EXECUTE FUNCTION preserve_project_knowledge_policy();

CREATE OR REPLACE FUNCTION collaboration_preserve_knowledge_automation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR TG_TABLE_NAME = 'collaboration_knowledge_evaluation' THEN
        RAISE EXCEPTION 'knowledge automation history must be retained' USING ERRCODE = '23514';
    END IF;
    IF TG_TABLE_NAME = 'collaboration_knowledge_job' THEN
        IF (to_jsonb(NEW) - ARRAY['phase','reason_code','batch_manifest','execution_revision','db_retry_count','next_attempt_at','lease_token','lease_generation','lease_expires_at','result','updated_at','completed_at'])
           IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['phase','reason_code','batch_manifest','execution_revision','db_retry_count','next_attempt_at','lease_token','lease_generation','lease_expires_at','result','updated_at','completed_at'])
           OR (OLD.batch_manifest IS NOT NULL AND NEW.batch_manifest IS DISTINCT FROM OLD.batch_manifest)
           OR NEW.execution_revision NOT IN (OLD.execution_revision, OLD.execution_revision + 1)
           OR NEW.lease_generation NOT IN (OLD.lease_generation, OLD.lease_generation + 1)
           OR (OLD.phase IN ('completed','skipped','cancelled') AND NEW IS DISTINCT FROM OLD) THEN
            RAISE EXCEPTION 'knowledge job identity and settled results are immutable' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_knowledge_batch' THEN
        IF ROW(NEW.id,NEW.job_id,NEW.batch_index,NEW.intent,NEW.predecessor_ids,NEW.created_at)
           IS DISTINCT FROM ROW(OLD.id,OLD.job_id,OLD.batch_index,OLD.intent,OLD.predecessor_ids,OLD.created_at)
           OR NEW.execution_revision NOT IN (OLD.execution_revision, OLD.execution_revision + 1)
           OR (OLD.phase IN ('completed','skipped','cancelled') AND NEW IS DISTINCT FROM OLD) THEN
            RAISE EXCEPTION 'knowledge batch identity and settled results are immutable' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_knowledge_attempt' THEN
        IF (to_jsonb(NEW) - ARRAY['state','response_json','actual_model','usage','reason_code','finished_at'])
           IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['state','response_json','actual_model','usage','reason_code','finished_at'])
           OR OLD.state <> 'running' OR NEW.state = 'running' THEN
            RAISE EXCEPTION 'knowledge model attempt input and terminal result are immutable' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'collaboration_knowledge_validity' THEN
        IF ROW(NEW.candidate_id,NEW.source_issue_id,NEW.project_id) IS DISTINCT FROM ROW(OLD.candidate_id,OLD.source_issue_id,OLD.project_id)
           OR NEW.revision <> OLD.revision + 1 OR NEW.evaluation_id = OLD.evaluation_id
           OR NEW.accepted_event_seq <= OLD.accepted_event_seq OR NEW.validated_close_epoch <= OLD.validated_close_epoch THEN
            RAISE EXCEPTION 'invalid knowledge validity revision' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS collaboration_knowledge_job_immutable ON collaboration_knowledge_job;
CREATE TRIGGER collaboration_knowledge_job_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_job FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_automation();
DROP TRIGGER IF EXISTS collaboration_knowledge_batch_immutable ON collaboration_knowledge_batch;
CREATE TRIGGER collaboration_knowledge_batch_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_batch FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_automation();
DROP TRIGGER IF EXISTS collaboration_knowledge_attempt_immutable ON collaboration_knowledge_attempt;
CREATE TRIGGER collaboration_knowledge_attempt_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_attempt FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_automation();
DROP TRIGGER IF EXISTS collaboration_knowledge_evaluation_immutable ON collaboration_knowledge_evaluation;
CREATE TRIGGER collaboration_knowledge_evaluation_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_evaluation FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_automation();
DROP TRIGGER IF EXISTS collaboration_knowledge_validity_immutable ON collaboration_knowledge_validity;
CREATE TRIGGER collaboration_knowledge_validity_immutable BEFORE UPDATE OR DELETE ON collaboration_knowledge_validity FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_knowledge_automation();

CREATE OR REPLACE FUNCTION collaboration_assert_knowledge_automation_empty() RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    retained_event record;
    required_keys text[];
    request_payload jsonb;
    manifest_payload jsonb;
    manifest_entry jsonb;
BEGIN
    LOCK TABLE project_knowledge_policy, collaboration_knowledge_job, collaboration_knowledge_batch, collaboration_knowledge_attempt,
        collaboration_knowledge_evaluation, collaboration_knowledge_validity, collaboration_knowledge_candidate, agent_task_queue,
        collaboration_operation, collaboration_run_event IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM project_knowledge_policy) OR EXISTS (SELECT 1 FROM collaboration_knowledge_job)
       OR EXISTS (SELECT 1 FROM collaboration_knowledge_batch) OR EXISTS (SELECT 1 FROM collaboration_knowledge_attempt)
       OR EXISTS (SELECT 1 FROM collaboration_knowledge_evaluation) OR EXISTS (SELECT 1 FROM collaboration_knowledge_validity)
       OR EXISTS (SELECT 1 FROM collaboration_knowledge_candidate WHERE proposal_origin <> 'human' OR decision_origin <> 'human' OR approval_origin <> 'human')
       OR EXISTS (SELECT 1 FROM collaboration_knowledge_candidate c WHERE
           EXISTS (SELECT 1 FROM jsonb_array_elements(c.sources) AS required_source(value)
               WHERE COALESCE(required_source.value->>'kind','') NOT IN ('memory','comment','run_record'))
           OR EXISTS (SELECT 1 FROM jsonb_array_elements(c.evidence) AS required_evidence(value) WHERE required_evidence.value ? 'source'
               AND COALESCE(required_evidence.value->'source'->>'kind','') NOT IN ('memory','comment','run_record')))
       OR EXISTS (SELECT 1 FROM agent_task_queue WHERE knowledge_context_version = 2)
       OR EXISTS (SELECT 1 FROM collaboration_operation WHERE scope_kind = 'project')
       OR EXISTS (SELECT 1 FROM collaboration_operation WHERE starts_with(kind,'knowledge_') AND
           (kind NOT IN ('knowledge_propose','knowledge_approve','knowledge_reject','knowledge_withdraw')
            OR result IS NOT NULL AND CASE WHEN jsonb_typeof(result) <> 'object' THEN true ELSE
                result->'protocol_version' IS DISTINCT FROM '1'::jsonb
                OR result->>'protocol_version' IS DISTINCT FROM '1'
                OR NOT result ?& ARRAY['protocol_version','request_id','action','code','candidate_id','version','state_revision','state']
                OR result - ARRAY['protocol_version','request_id','action','code','candidate_id','version','state_revision','state','target_knowledge_revision'] <> '{}'::jsonb
            END)) THEN
        RAISE EXCEPTION 'refusing rollback: retained knowledge automation state exists' USING ERRCODE = '55000';
    END IF;
    -- The historical slot names remain stable across protocol versions. Only
    -- known v1 envelopes may survive a downgrade; missing or unknown shapes
    -- are not evidence that the run has no retained knowledge exposure.
    FOR retained_event IN SELECT kind,producer_event_id,payload FROM collaboration_run_event
        WHERE starts_with(kind,'knowledge_') OR starts_with(producer_event_id,'knowledge:') LOOP
        IF jsonb_typeof(retained_event.payload) IS DISTINCT FROM 'object' THEN
            RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
        END IF;
        request_payload := retained_event.payload;
        CASE
        WHEN retained_event.kind = 'knowledge_claim' AND retained_event.producer_event_id = 'knowledge:claim:v1' THEN
            required_keys := ARRAY['protocol_version','identity','configuration_digest','namespace'];
        WHEN retained_event.kind = 'knowledge_authorization' AND retained_event.producer_event_id ~ '^knowledge:launch:[1-9][0-9]*$' THEN
            required_keys := ARRAY['protocol_version','identity','request_id','handoff_id','invocation_fingerprint','manifest_sha256','prompt_sha256','attempt_seq','attempt_nonce'];
        WHEN retained_event.kind = 'knowledge_preparation' AND retained_event.producer_event_id = 'knowledge:prepare:v1' THEN
            required_keys := ARRAY['request','manifest','manifest_sha256','targets'];
            request_payload := retained_event.payload->'request';
            manifest_payload := retained_event.payload->'manifest';
            IF jsonb_typeof(request_payload) IS DISTINCT FROM 'object' OR jsonb_typeof(manifest_payload) IS DISTINCT FROM 'object'
               OR jsonb_typeof(retained_event.payload->'targets') IS DISTINCT FROM 'array' THEN
                RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
            END IF;
            IF NOT request_payload ?& ARRAY['protocol_version','identity','request_id','handoff_id','invocation_fingerprint']
               OR request_payload - ARRAY['protocol_version','identity','request_id','handoff_id','invocation_fingerprint'] <> '{}'::jsonb
               OR manifest_payload->'protocol_version' IS DISTINCT FROM '1'::jsonb
               OR manifest_payload->>'protocol_version' IS DISTINCT FROM '1'
               OR jsonb_typeof(manifest_payload->'entries') IS DISTINCT FROM 'array'
               OR manifest_payload - ARRAY['protocol_version','entries'] <> '{}'::jsonb THEN
                RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
            END IF;
            FOR manifest_entry IN SELECT value FROM jsonb_array_elements(manifest_payload->'entries') LOOP
                IF jsonb_typeof(manifest_entry) IS DISTINCT FROM 'object' THEN
                    RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
                END IF;
                IF NOT manifest_entry ?& ARRAY['candidate_id','version','content_sha256','source_issue_id','target']
                   OR manifest_entry - ARRAY['candidate_id','version','content_sha256','source_issue_id','target'] <> '{}'::jsonb THEN
                    RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
                END IF;
            END LOOP;
        ELSE
            RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
        END CASE;
        IF NOT retained_event.payload ?& required_keys OR retained_event.payload - required_keys <> '{}'::jsonb
           OR request_payload->'protocol_version' IS DISTINCT FROM '1'::jsonb
           OR request_payload->>'protocol_version' IS DISTINCT FROM '1'
           OR jsonb_typeof(request_payload->'identity') IS DISTINCT FROM 'object' THEN
            RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
        END IF;
        IF NOT request_payload->'identity' ?& ARRAY['deployment_id','installation_id','issue_id','session_id','target_id','channel_id','batch_id','group_id','run_id','fence','claim_generation','launch_nonce']
           OR (request_payload->'identity') - ARRAY['deployment_id','installation_id','issue_id','session_id','target_id','channel_id','batch_id','group_id','run_id','fence','claim_generation','launch_nonce'] <> '{}'::jsonb
           OR EXISTS (SELECT 1 FROM jsonb_each(request_payload->'identity') WHERE jsonb_typeof(value) <> 'string')
           OR EXISTS (SELECT 1 FROM jsonb_each(request_payload) WHERE key NOT IN ('protocol_version','identity') AND jsonb_typeof(value) <> 'string') THEN
            RAISE EXCEPTION 'refusing rollback: unsupported retained knowledge runtime' USING ERRCODE = '55000';
        END IF;
    END LOOP;
END $$;
