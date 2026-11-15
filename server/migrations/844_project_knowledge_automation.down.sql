SELECT collaboration_assert_knowledge_automation_empty();

ALTER TABLE collaboration_operation DROP CONSTRAINT collaboration_operation_scope_kind_check;
ALTER TABLE collaboration_operation ADD CONSTRAINT collaboration_operation_scope_kind_check CHECK (scope_kind IN ('create','issue'));
ALTER TABLE agent_task_queue DROP CONSTRAINT agent_task_knowledge_metadata;
ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_knowledge_metadata CHECK (
    (knowledge_context_version IS NULL OR knowledge_context_version = 1)
    AND (knowledge_delivery IS NULL OR (knowledge_context_version IS NOT NULL AND jsonb_typeof(knowledge_delivery) = 'object' AND octet_length(knowledge_delivery::text) <= 65536)));
ALTER TABLE collaboration_knowledge_candidate DROP CONSTRAINT collaboration_knowledge_proposal_origin;
ALTER TABLE collaboration_knowledge_candidate DROP CONSTRAINT collaboration_knowledge_candidate_check6;
ALTER TABLE collaboration_knowledge_candidate DROP CONSTRAINT collaboration_knowledge_candidate_check9;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN proposal_origin;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN decision_origin;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN approval_origin;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN automation_job_id;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN automation_policy_revision;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN automation_rule_version;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN approval_job_id;
ALTER TABLE collaboration_knowledge_candidate DROP COLUMN last_decision_job_id;
ALTER TABLE collaboration_knowledge_candidate ALTER COLUMN proposed_by_user_id SET NOT NULL;
ALTER TABLE collaboration_knowledge_candidate ADD CONSTRAINT collaboration_knowledge_candidate_check6 CHECK (
    (approved_at IS NULL AND approved_by_user_id IS NULL AND published_target_revision IS NULL) OR (approved_at IS NOT NULL AND approved_by_user_id IS NOT NULL AND published_target_revision IS NOT NULL));
ALTER TABLE collaboration_knowledge_candidate ADD CONSTRAINT collaboration_knowledge_candidate_check9 CHECK (
    (state = 'pending' AND state_revision = 1 AND last_decided_by_user_id IS NULL AND last_decided_at IS NULL AND last_decision_reason IS NULL) OR (state <> 'pending' AND last_decided_by_user_id IS NOT NULL AND last_decided_at IS NOT NULL));

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


DROP TABLE collaboration_knowledge_validity;
DROP TABLE collaboration_knowledge_evaluation;
DROP TABLE collaboration_knowledge_attempt;
DROP TABLE collaboration_knowledge_batch;
DROP TABLE collaboration_knowledge_job;
DROP TABLE project_knowledge_policy;
DROP FUNCTION collaboration_preserve_knowledge_automation();
DROP FUNCTION preserve_project_knowledge_policy();
DROP FUNCTION collaboration_assert_knowledge_automation_empty();
