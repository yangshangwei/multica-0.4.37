-- Policy absence is resolved by the service as the built-in revision zero.
-- Authority and operation recovery use the existing project/member locks and
-- collaboration_operation(scope_kind='project'), never a last-request column.

-- A missing override row still needs a policy-generation fence against its
-- first INSERT. Both policy writers and settling readers acquire this before
-- target/source locks; acceptance intent insertion deliberately does not.
-- name: TryKnowledgePolicyFence :one
SELECT pg_try_advisory_xact_lock(hashtextextended(sqlc.arg(project_id)::text,81271466))::boolean;

-- name: GetProjectKnowledgePolicy :one
SELECT * FROM project_knowledge_policy WHERE project_id=$1 AND workspace_id=$2;

-- name: LockProjectKnowledgePolicy :one
SELECT * FROM project_knowledge_policy WHERE project_id=$1 AND workspace_id=$2 FOR UPDATE NOWAIT;

-- name: LockProjectKnowledgePolicyForRead :one
SELECT * FROM project_knowledge_policy WHERE project_id=$1 AND workspace_id=$2 FOR SHARE NOWAIT;

-- name: CreateProjectKnowledgePolicy :one
INSERT INTO project_knowledge_policy (project_id,workspace_id,is_enabled,max_candidates,builtin_rule_version,changed_by_user_id)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (project_id) DO NOTHING RETURNING *;

-- name: UpdateProjectKnowledgePolicy :one
UPDATE project_knowledge_policy SET revision=revision+1,is_enabled=sqlc.arg(is_enabled),max_candidates=sqlc.arg(max_candidates),
    builtin_rule_version=sqlc.arg(builtin_rule_version),changed_by_user_id=sqlc.arg(changed_by_user_id),updated_at=clock_timestamp()
WHERE project_id=sqlc.arg(project_id) AND workspace_id=sqlc.arg(workspace_id) AND revision=sqlc.arg(expected_revision) RETURNING *;

-- Only parent deletion transactions may remove target-owned configuration.
-- name: DeleteProjectKnowledgePolicy :execrows
DELETE FROM project_knowledge_policy WHERE project_id=$1 AND workspace_id=$2;

-- name: DeleteWorkspaceKnowledgePolicies :execrows
DELETE FROM project_knowledge_policy WHERE workspace_id=$1;

-- The acceptance transaction adds only this lightweight intent. No target
-- knowledge lock, model request or original business-queue work is involved.
-- name: CreateKnowledgeAutomationJob :one
INSERT INTO collaboration_knowledge_job (id,source_workspace_id,source_issue_id,project_id,submission_id,delivery_event_seq,accepted_event_seq,
    validated_close_epoch,source_reader_user_id,policy_revision,builtin_rule_version,max_candidates,execution_policy_revision,execution_rule_version,execution_max_candidates)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$10,$11,$12)
ON CONFLICT (source_issue_id,accepted_event_seq) DO NOTHING RETURNING *;

-- name: GetKnowledgeAutomationJob :one
SELECT * FROM collaboration_knowledge_job WHERE id=$1;

-- name: GetKnowledgeAutomationJobForAcceptance :one
SELECT * FROM collaboration_knowledge_job WHERE source_issue_id=$1 AND accepted_event_seq=$2;

-- name: ListKnowledgeAutomationJobs :many
SELECT * FROM collaboration_knowledge_job WHERE source_workspace_id=$1 AND source_issue_id=$2
ORDER BY accepted_event_seq DESC LIMIT $3;

-- name: LockKnowledgeAutomationJob :one
SELECT * FROM collaboration_knowledge_job WHERE id=$1 FOR UPDATE NOWAIT;

-- A crashed model request is not automatically repeated after taking its job.
-- The new owner first seals old running attempts as uncertain, then projects
-- review_pending. Successful attempt output remains readable by its exact ID.
-- name: ClaimKnowledgeAutomationJob :one
WITH next_job AS (
    SELECT id FROM collaboration_knowledge_job
    WHERE phase IN ('queued','generating','checking','settling','waiting_configuration')
      AND next_attempt_at<=clock_timestamp() AND (lease_expires_at IS NULL OR lease_expires_at<=clock_timestamp())
    ORDER BY next_attempt_at,id LIMIT 1 FOR UPDATE SKIP LOCKED
)
UPDATE collaboration_knowledge_job j SET lease_token=sqlc.arg(lease_token)::uuid,lease_generation=lease_generation+1,
    lease_expires_at=clock_timestamp()+interval '180 seconds',updated_at=clock_timestamp()
FROM next_job WHERE j.id=next_job.id RETURNING j.*;

-- name: FreezeKnowledgeAutomationBatches :one
UPDATE collaboration_knowledge_job SET batch_manifest=sqlc.arg(batch_manifest)::jsonb,updated_at=clock_timestamp()
WHERE id=sqlc.arg(id) AND batch_manifest IS NULL AND lease_token=sqlc.arg(lease_token)
  AND lease_generation=sqlc.arg(lease_generation) AND lease_expires_at>clock_timestamp() RETURNING *;

-- name: CreateKnowledgeAutomationBatch :one
INSERT INTO collaboration_knowledge_batch (id,job_id,batch_index,intent,predecessor_ids,execution_revision)
SELECT sqlc.arg(id)::uuid,j.id,sqlc.arg(batch_index)::smallint,sqlc.arg(intent)::text,sqlc.arg(predecessor_ids)::uuid[],j.execution_revision
FROM collaboration_knowledge_job j WHERE j.id=sqlc.arg(job_id) AND j.lease_token=sqlc.arg(lease_token)
  AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp()
ON CONFLICT (job_id,batch_index) DO NOTHING RETURNING *;

-- name: ListKnowledgeAutomationBatches :many
SELECT * FROM collaboration_knowledge_batch WHERE job_id=$1 ORDER BY batch_index;

-- name: GetKnowledgeAutomationBatch :one
SELECT * FROM collaboration_knowledge_batch WHERE id=$1 AND job_id=$2;

-- name: UpdateKnowledgeAutomationBatch :one
UPDATE collaboration_knowledge_batch b SET phase=sqlc.arg(phase),generation_attempt_id=sqlc.narg(generation_attempt_id)::uuid,
    check_attempt_id=sqlc.narg(check_attempt_id)::uuid,result=sqlc.narg(result)::jsonb,updated_at=clock_timestamp(),
    completed_at=CASE WHEN sqlc.arg(phase)::text IN ('completed','review_pending','skipped','cancelled') THEN clock_timestamp() ELSE NULL END
FROM collaboration_knowledge_job j WHERE b.id=sqlc.arg(id) AND j.id=b.job_id AND j.id=sqlc.arg(job_id)
  AND b.execution_revision=j.execution_revision AND b.execution_revision=sqlc.arg(execution_revision)
  AND b.phase NOT IN ('completed','skipped','cancelled') AND j.lease_token=sqlc.arg(lease_token)
  AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp() RETURNING b.*;

-- name: UpdateKnowledgeAutomationJob :one
UPDATE collaboration_knowledge_job SET phase=sqlc.arg(phase),reason_code=sqlc.narg(reason_code)::text,
    result=sqlc.narg(result)::jsonb,updated_at=clock_timestamp()
WHERE id=sqlc.arg(id) AND execution_revision=sqlc.arg(execution_revision) AND lease_token=sqlc.arg(lease_token)
  AND lease_generation=sqlc.arg(lease_generation) AND lease_expires_at>clock_timestamp()
  AND sqlc.arg(phase)::text IN ('queued','generating','checking','settling','waiting_configuration')
  AND phase NOT IN ('completed','skipped','cancelled') RETURNING *;

-- name: ReleaseKnowledgeAutomationJob :one
UPDATE collaboration_knowledge_job SET phase=sqlc.arg(phase),reason_code=sqlc.narg(reason_code)::text,
    result=sqlc.narg(result)::jsonb,db_retry_count=sqlc.arg(db_retry_count),next_attempt_at=sqlc.arg(next_attempt_at),
    lease_token=NULL,lease_expires_at=NULL,updated_at=clock_timestamp(),
    completed_at=CASE WHEN sqlc.arg(phase)::text IN ('completed','review_pending','skipped','cancelled') THEN clock_timestamp() ELSE NULL END
WHERE id=sqlc.arg(id) AND execution_revision=sqlc.arg(execution_revision) AND lease_token=sqlc.arg(lease_token)
  AND lease_generation=sqlc.arg(lease_generation) AND lease_expires_at>clock_timestamp()
  AND phase NOT IN ('completed','skipped','cancelled') RETURNING *;

-- Explicit human retry is an operation-receipt transaction, never a worker loop.
-- name: RetryKnowledgeAutomationJob :one
UPDATE collaboration_knowledge_job SET phase='queued',reason_code=NULL,result=NULL,execution_revision=execution_revision+1,
    execution_policy_revision=sqlc.arg(policy_revision),execution_rule_version=sqlc.arg(rule_version),execution_max_candidates=sqlc.arg(max_candidates),
    db_retry_count=0,next_attempt_at=clock_timestamp(),lease_token=NULL,lease_expires_at=NULL,completed_at=NULL,updated_at=clock_timestamp()
WHERE id=sqlc.arg(id) AND execution_revision=sqlc.arg(expected_execution_revision)
  AND phase IN ('review_pending','waiting_configuration') AND (lease_expires_at IS NULL OR lease_expires_at<=clock_timestamp()) RETURNING *;

-- name: ResetKnowledgeAutomationBatchForRetry :execrows
UPDATE collaboration_knowledge_batch b SET phase='queued',execution_revision=j.execution_revision,
    generation_attempt_id=NULL,check_attempt_id=NULL,completed_at=NULL,updated_at=clock_timestamp()
FROM collaboration_knowledge_job j WHERE b.job_id=j.id AND j.id=sqlc.arg(job_id)
  AND j.execution_revision=sqlc.arg(execution_revision) AND b.execution_revision=j.execution_revision-1
  AND b.phase NOT IN ('completed','skipped','cancelled');

-- name: CreateKnowledgeModelAttempt :one
INSERT INTO collaboration_knowledge_attempt (id,job_id,batch_id,execution_revision,stage,attempt_number,lease_generation,
    configuration_fingerprint,requested_model,source_snapshot_sha256,target_snapshot_sha256,target_revision,policy_revision,
    request_sha256,input_source_closure,support_dependencies)
SELECT sqlc.arg(id)::uuid,j.id,b.id,j.execution_revision,sqlc.arg(stage)::text,sqlc.arg(attempt_number)::smallint,j.lease_generation,
    sqlc.arg(configuration_fingerprint)::text,sqlc.arg(requested_model)::text,sqlc.arg(source_snapshot_sha256)::text,
    sqlc.arg(target_snapshot_sha256)::text,sqlc.arg(target_revision)::bigint,j.execution_policy_revision,sqlc.arg(request_sha256)::text,
    sqlc.arg(input_source_closure)::jsonb,sqlc.arg(support_dependencies)::jsonb
FROM collaboration_knowledge_job j JOIN collaboration_knowledge_batch b ON b.job_id=j.id
WHERE j.id=sqlc.arg(job_id) AND b.id=sqlc.arg(batch_id) AND j.execution_revision=b.execution_revision
  AND j.execution_revision=sqlc.arg(execution_revision) AND j.lease_token=sqlc.arg(lease_token)
  AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp()
  AND b.phase IN ('queued','generating','checking')
ON CONFLICT (batch_id,execution_revision,stage,attempt_number) DO NOTHING RETURNING *;

-- name: GetKnowledgeModelAttempt :one
SELECT * FROM collaboration_knowledge_attempt WHERE id=$1 AND job_id=$2;

-- name: ListKnowledgeModelAttempts :many
SELECT * FROM collaboration_knowledge_attempt WHERE batch_id=$1 ORDER BY execution_revision,stage,attempt_number;

-- Workers only resume the bounded current execution, never an unbounded
-- transcript of previous explicitly requested retries.
-- name: ListKnowledgeExecutionModelAttempts :many
SELECT * FROM collaboration_knowledge_attempt WHERE batch_id=$1 AND execution_revision=$2 ORDER BY stage,attempt_number;

-- Public job status never loads model output or private source closures.
-- name: ListKnowledgeExecutionModelUsage :many
SELECT stage,state,actual_model,usage FROM collaboration_knowledge_attempt
WHERE job_id=$1 AND execution_revision=$2 ORDER BY batch_id,stage,attempt_number LIMIT 43;

-- name: FinishKnowledgeModelAttempt :one
UPDATE collaboration_knowledge_attempt a SET state=sqlc.arg(state),response_json=sqlc.narg(response_json)::jsonb,
    actual_model=sqlc.narg(actual_model)::text,usage=sqlc.arg(usage)::jsonb,reason_code=sqlc.narg(reason_code)::text,finished_at=clock_timestamp()
FROM collaboration_knowledge_job j WHERE a.id=sqlc.arg(id) AND a.job_id=j.id AND j.id=sqlc.arg(job_id)
  AND a.state='running' AND a.execution_revision=j.execution_revision AND a.execution_revision=sqlc.arg(execution_revision)
  AND a.lease_generation=j.lease_generation AND j.lease_generation=sqlc.arg(lease_generation)
  AND j.lease_token=sqlc.arg(lease_token) AND j.lease_expires_at>clock_timestamp() RETURNING a.*;

-- name: MarkKnowledgeAbandonedAttemptsUncertain :execrows
UPDATE collaboration_knowledge_attempt a SET state='uncertain',usage='{"status":"unknown"}'::jsonb,
    reason_code='model_attempt_uncertain',finished_at=clock_timestamp()
FROM collaboration_knowledge_job j WHERE a.job_id=j.id AND j.id=sqlc.arg(job_id) AND a.state='running'
  AND a.lease_generation<j.lease_generation AND j.lease_token=sqlc.arg(lease_token)
  AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp();

-- All source and target authority has already been locked/rechecked by the
-- settling transaction. Persist the actual check request's complete closure,
-- independently of whatever related_refs the model included in its output.
-- name: CreateKnowledgeEvaluation :one
INSERT INTO collaboration_knowledge_evaluation (id,job_id,batch_id,generation_attempt_id,check_attempt_id,candidate_id,
    candidate_version,candidate_content_sha256,intent,predecessor_candidate_id,source_workspace_id,source_issue_id,project_id,
    submission_id,accepted_event_seq,validated_close_epoch,policy_revision,builtin_rule_version,target_revision,
    source_snapshot_sha256,target_snapshot_sha256,verdict,reason_code,input_source_closure,support_dependencies,model_result,usage)
SELECT sqlc.arg(id)::uuid,j.id,b.id,g.id,k.id,sqlc.arg(candidate_id)::uuid,sqlc.arg(candidate_version)::bigint,
    sqlc.arg(candidate_content_sha256)::text,sqlc.arg(intent)::text,sqlc.narg(predecessor_candidate_id)::uuid,j.source_workspace_id,
    j.source_issue_id,j.project_id,j.submission_id,j.accepted_event_seq,j.validated_close_epoch,j.execution_policy_revision,j.execution_rule_version,
    k.target_revision,k.source_snapshot_sha256,k.target_snapshot_sha256,sqlc.arg(verdict)::text,sqlc.arg(reason_code)::text,
    k.input_source_closure,sqlc.arg(support_dependencies)::jsonb,sqlc.arg(model_result)::jsonb,
    jsonb_build_object('generation',g.usage,'checking',k.usage)
FROM collaboration_knowledge_job j JOIN collaboration_knowledge_batch b ON b.job_id=j.id
JOIN collaboration_knowledge_attempt g ON g.id=b.generation_attempt_id AND g.batch_id=b.id AND g.job_id=j.id AND g.stage='generating'
JOIN collaboration_knowledge_attempt k ON k.id=b.check_attempt_id AND k.batch_id=b.id AND k.job_id=j.id AND k.stage='checking'
WHERE j.id=sqlc.arg(job_id) AND b.id=sqlc.arg(batch_id) AND g.state='succeeded' AND k.state='succeeded'
  AND g.execution_revision=j.execution_revision AND k.execution_revision=j.execution_revision AND b.execution_revision=j.execution_revision
  AND g.source_snapshot_sha256=k.source_snapshot_sha256 AND g.target_snapshot_sha256=k.target_snapshot_sha256
  AND g.target_revision=k.target_revision AND g.policy_revision=j.execution_policy_revision AND k.policy_revision=j.execution_policy_revision
  AND j.lease_token=sqlc.arg(lease_token) AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp()
ON CONFLICT (check_attempt_id,candidate_id) DO NOTHING RETURNING *;

-- name: GetKnowledgeEvaluation :one
SELECT * FROM collaboration_knowledge_evaluation WHERE id=$1;

-- name: ListKnowledgeCandidateEvaluations :many
SELECT * FROM collaboration_knowledge_evaluation WHERE candidate_id=$1 ORDER BY created_at,id;

-- A still-approved automatic candidate and a passing exact proof are required
-- even after the caller has locked them. Human withdrawal cannot be resurrected.
-- name: SetKnowledgeValidity :one
INSERT INTO collaboration_knowledge_validity (candidate_id,evaluation_id,source_issue_id,project_id,submission_id,accepted_event_seq,validated_close_epoch)
SELECT c.id,e.id,e.source_issue_id,e.project_id,e.submission_id,e.accepted_event_seq,e.validated_close_epoch
FROM collaboration_knowledge_candidate c JOIN collaboration_knowledge_evaluation e ON e.candidate_id=c.id
WHERE c.id=sqlc.arg(candidate_id) AND e.id=sqlc.arg(evaluation_id) AND c.state='approved' AND c.approval_origin='automation'
  AND c.version=e.candidate_version AND c.content_sha256=e.candidate_content_sha256 AND c.source_issue_id=e.source_issue_id
  AND c.target_project_id=e.project_id AND c.state_revision=sqlc.arg(expected_state_revision) AND e.verdict='pass'
  AND ((sqlc.arg(expected_revision)::bigint>0 AND EXISTS (SELECT 1 FROM collaboration_knowledge_validity WHERE candidate_id=c.id AND revision=sqlc.arg(expected_revision)))
    OR (sqlc.arg(expected_revision)::bigint=0 AND NOT EXISTS (SELECT 1 FROM collaboration_knowledge_validity WHERE candidate_id=c.id)))
ON CONFLICT (candidate_id) DO UPDATE SET revision=collaboration_knowledge_validity.revision+1,evaluation_id=EXCLUDED.evaluation_id,
    submission_id=EXCLUDED.submission_id,accepted_event_seq=EXCLUDED.accepted_event_seq,
    validated_close_epoch=EXCLUDED.validated_close_epoch,updated_at=clock_timestamp()
WHERE collaboration_knowledge_validity.revision=sqlc.arg(expected_revision)
  AND collaboration_knowledge_validity.evaluation_id<>EXCLUDED.evaluation_id RETURNING *;

-- name: GetKnowledgeValidity :one
SELECT * FROM collaboration_knowledge_validity WHERE candidate_id=$1;

-- name: LockKnowledgeValidity :one
SELECT * FROM collaboration_knowledge_validity WHERE candidate_id=$1 FOR UPDATE NOWAIT;

-- The LEFT JOIN retains a corrupt/missing evaluation as an explicit missing
-- dependency, rather than silently treating the proof as absent legacy history.
-- name: ListKnowledgeValidityLockHints :many
SELECT v.candidate_id,v.revision,v.evaluation_id,v.source_issue_id,v.project_id,v.submission_id,v.accepted_event_seq,
    v.validated_close_epoch,e.support_dependencies
FROM collaboration_knowledge_validity v LEFT JOIN collaboration_knowledge_evaluation e ON e.id=v.evaluation_id
WHERE v.candidate_id=ANY(sqlc.arg(candidate_ids)::uuid[]) ORDER BY v.candidate_id;

-- Expired candidates remain in the complete frozen review inventory. Services
-- decide their review outcome; a LIMIT 3 or expiry filter would lose predecessors.
-- name: ListKnowledgeAutomationRevalidationCandidates :many
SELECT * FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 AND target_project_id=$2
  AND target_workspace_id=$3 AND state='approved' AND approval_origin='automation' ORDER BY id LIMIT 17;

-- Acquire the complete eligible comparison or an explicit overflow marker,
-- without loading unbounded expired history into the worker.
-- name: ListKnowledgeAutomationComparisonRows :many
SELECT * FROM collaboration_knowledge_candidate WHERE target_kind='project' AND target_project_id=$1
  AND target_workspace_id=$2 AND state='approved'
  AND (expires_at IS NULL OR expires_at>sqlc.arg(checked_at)::timestamptz OR id=ANY(sqlc.arg(predecessor_ids)::uuid[]))
ORDER BY id LIMIT 17;

-- Count logical independent lineages ever published from this acceptance,
-- including later manual approval and withdrawn/superseded published history.
-- name: CountKnowledgeAutomationPublishedLineages :one
SELECT count(DISTINCT root.lineage_id)::bigint
FROM collaboration_knowledge_candidate root
WHERE root.automation_job_id=$1 AND root.id=root.lineage_id AND root.proposal_origin='automation'
  AND EXISTS (SELECT 1 FROM collaboration_knowledge_candidate published
    WHERE published.lineage_id=root.lineage_id AND published.published_target_revision IS NOT NULL);

-- name: ListKnowledgeApprovedTargetCandidates :many
SELECT * FROM collaboration_knowledge_candidate WHERE target_kind='project' AND target_project_id=$1
  AND target_workspace_id=$2 AND state='approved' ORDER BY id;

-- A fresh candidate ID must not bypass a human disposition of the same
-- automatically proposed knowledge. This examines retained metadata in-place;
-- no historical rejected/withdrawn text is sent back to the model.
-- name: KnowledgeAutomationHasHumanDisposition :one
SELECT EXISTS (
    SELECT 1 FROM collaboration_knowledge_candidate
    WHERE source_issue_id=sqlc.arg(source_issue_id) AND target_project_id=sqlc.arg(project_id)
      AND target_workspace_id=sqlc.arg(workspace_id) AND proposal_origin='automation' AND decision_origin='human'
      AND state IN ('withdrawn','rejected','superseded')
      AND replace(replace(content,E'\r\n',E'\n'),E'\r',E'\n')=replace(replace(sqlc.arg(content)::text,E'\r\n',E'\n'),E'\r',E'\n')
      AND applicability=sqlc.arg(applicability)::jsonb
)::boolean;

-- Untouched automatic pending proposals are retained history, not human
-- decisions. A newer human correction or actual human disposition still wins.
-- name: KnowledgeLineageHasNewerHumanDecision :one
SELECT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate
    WHERE lineage_id=$1 AND version>$2
      AND (proposal_origin='human' OR (state<>'pending' AND decision_origin='human')))::boolean;

-- System proposal is private to the settlement service. It cannot impersonate
-- the acceptance reader and can retain valid review_pending model candidates.
-- name: CreateAutomatedKnowledgeCandidate :one
INSERT INTO collaboration_knowledge_candidate (id,lineage_id,version,source_workspace_id,source_issue_id,target_kind,target_workspace_id,
    target_project_id,content,summary,content_sha256,sources,evidence,applicability,expires_at,supersedes_candidate_id,
    proposal_origin,automation_job_id,automation_policy_revision,automation_rule_version)
SELECT sqlc.arg(id)::uuid,COALESCE(p.lineage_id,sqlc.arg(id)::uuid),
    (COALESCE((SELECT max(prior.version) FROM collaboration_knowledge_candidate prior WHERE prior.lineage_id=p.lineage_id),0)+1)::bigint,
    j.source_workspace_id,j.source_issue_id,'project',j.source_workspace_id,j.project_id,sqlc.arg(content)::text,sqlc.arg(summary)::text,
    sqlc.arg(content_sha256)::text,sqlc.arg(sources)::jsonb,sqlc.arg(evidence)::jsonb,sqlc.arg(applicability)::jsonb,
    sqlc.narg(expires_at)::timestamptz,sqlc.narg(supersedes_candidate_id)::uuid,'automation',j.id,j.execution_policy_revision,j.execution_rule_version
FROM collaboration_knowledge_job j LEFT JOIN collaboration_knowledge_candidate p ON p.id=sqlc.narg(supersedes_candidate_id)::uuid
WHERE j.id=sqlc.arg(job_id) AND j.lease_token=sqlc.arg(lease_token) AND j.lease_generation=sqlc.arg(lease_generation)
  AND j.lease_expires_at>clock_timestamp() AND
    ((sqlc.narg(supersedes_candidate_id)::uuid IS NULL AND sqlc.arg(expected_previous_version)::bigint=0)
    OR (p.source_issue_id=j.source_issue_id AND p.source_workspace_id=j.source_workspace_id AND p.target_project_id=j.project_id
      AND p.target_workspace_id=j.source_workspace_id AND p.state='approved' AND p.approval_origin='automation'
      AND p.version=sqlc.arg(expected_previous_version) AND p.state_revision=sqlc.arg(expected_previous_state_revision)
      AND NOT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate newer WHERE newer.lineage_id=p.lineage_id AND newer.version>p.version
        AND (newer.proposal_origin='human' OR (newer.state<>'pending' AND newer.decision_origin='human')))))
RETURNING *;

-- name: ApproveAutomatedKnowledgeCandidate :one
UPDATE collaboration_knowledge_candidate c SET state='approved',state_revision=c.state_revision+1,
    approval_origin='automation',approved_by_user_id=NULL,approved_at=clock_timestamp(),approval_job_id=j.id,
    decision_origin='automation',last_decided_by_user_id=NULL,last_decided_at=clock_timestamp(),last_decision_job_id=j.id,
    published_target_revision=sqlc.arg(target_revision)
FROM collaboration_knowledge_job j,collaboration_knowledge_evaluation e
WHERE c.id=sqlc.arg(id) AND c.state='pending' AND c.proposal_origin='automation' AND c.automation_job_id=j.id
  AND c.state_revision=sqlc.arg(expected_state_revision) AND c.version=sqlc.arg(expected_version)
  AND c.content_sha256=sqlc.arg(expected_content_sha256) AND (c.expires_at IS NULL OR c.expires_at>clock_timestamp())
  AND j.id=sqlc.arg(job_id) AND e.id=sqlc.arg(evaluation_id) AND e.job_id=j.id AND e.candidate_id=c.id
  AND e.candidate_version=c.version AND e.candidate_content_sha256=c.content_sha256 AND e.verdict='pass'
  AND j.lease_token=sqlc.arg(lease_token) AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp()
RETURNING c.*;

-- name: SupersedeAutomatedKnowledgeCandidate :one
UPDATE collaboration_knowledge_candidate c SET state='superseded',state_revision=c.state_revision+1,
    decision_origin='automation',last_decided_by_user_id=NULL,last_decided_at=clock_timestamp(),last_decision_job_id=j.id
FROM collaboration_knowledge_job j,collaboration_knowledge_candidate replacement
WHERE c.id=sqlc.arg(id) AND c.state='approved' AND c.approval_origin='automation' AND c.state_revision=sqlc.arg(expected_state_revision)
  AND replacement.id=sqlc.arg(replacement_id) AND replacement.supersedes_candidate_id=c.id AND replacement.lineage_id=c.lineage_id
  AND replacement.state='pending' AND replacement.proposal_origin='automation' AND replacement.automation_job_id=j.id
  AND j.id=sqlc.arg(job_id) AND j.source_issue_id=c.source_issue_id AND j.project_id=c.target_project_id
  AND j.lease_token=sqlc.arg(lease_token) AND j.lease_generation=sqlc.arg(lease_generation) AND j.lease_expires_at>clock_timestamp()
RETURNING c.*;
