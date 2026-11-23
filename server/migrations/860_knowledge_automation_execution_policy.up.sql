-- The original acceptance-policy snapshot is retained. Only an explicit
-- retry revision may select a new current policy for the next model attempt.
-- This single migration runs as one implicit transaction; ALTER TABLE's
-- exclusive lock protects the deterministic backfill from concurrent writers.
ALTER TABLE collaboration_knowledge_job ADD COLUMN IF NOT EXISTS execution_policy_revision bigint CHECK (execution_policy_revision >= 0);
ALTER TABLE collaboration_knowledge_job ADD COLUMN IF NOT EXISTS execution_rule_version text CHECK (octet_length(execution_rule_version) BETWEEN 1 AND 64);
ALTER TABLE collaboration_knowledge_job ADD COLUMN IF NOT EXISTS execution_max_candidates smallint CHECK (execution_max_candidates BETWEEN 1 AND 3);
ALTER TABLE collaboration_knowledge_job DISABLE TRIGGER collaboration_knowledge_job_immutable;
UPDATE collaboration_knowledge_job SET execution_policy_revision=policy_revision,execution_rule_version=builtin_rule_version,execution_max_candidates=max_candidates
WHERE execution_policy_revision IS NULL;
ALTER TABLE collaboration_knowledge_job ENABLE TRIGGER collaboration_knowledge_job_immutable;
ALTER TABLE collaboration_knowledge_job ALTER COLUMN execution_policy_revision SET NOT NULL;
ALTER TABLE collaboration_knowledge_job ALTER COLUMN execution_rule_version SET NOT NULL;
ALTER TABLE collaboration_knowledge_job ALTER COLUMN execution_max_candidates SET NOT NULL;

CREATE OR REPLACE FUNCTION collaboration_preserve_knowledge_automation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR TG_TABLE_NAME = 'collaboration_knowledge_evaluation' THEN
        RAISE EXCEPTION 'knowledge automation history must be retained' USING ERRCODE = '23514';
    END IF;
    IF TG_TABLE_NAME = 'collaboration_knowledge_job' THEN
        IF (to_jsonb(NEW) - ARRAY['phase','reason_code','batch_manifest','execution_policy_revision','execution_rule_version','execution_max_candidates','execution_revision','db_retry_count','next_attempt_at','lease_token','lease_generation','lease_expires_at','result','updated_at','completed_at'])
           IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['phase','reason_code','batch_manifest','execution_policy_revision','execution_rule_version','execution_max_candidates','execution_revision','db_retry_count','next_attempt_at','lease_token','lease_generation','lease_expires_at','result','updated_at','completed_at'])
           OR (OLD.batch_manifest IS NOT NULL AND NEW.batch_manifest IS DISTINCT FROM OLD.batch_manifest)
           OR NEW.execution_revision NOT IN (OLD.execution_revision, OLD.execution_revision + 1)
           OR (NEW.execution_revision = OLD.execution_revision AND ROW(NEW.execution_policy_revision,NEW.execution_rule_version,NEW.execution_max_candidates)
               IS DISTINCT FROM ROW(OLD.execution_policy_revision,OLD.execution_rule_version,OLD.execution_max_candidates))
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
