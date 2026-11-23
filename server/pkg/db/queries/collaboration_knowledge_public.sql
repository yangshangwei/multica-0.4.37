-- Check the entire authorized scope before serving any v1 page. Filtering only
-- the current page would silently hide new provenance or strand newer history.
-- name: KnowledgeSourceRequiresV2 :one
SELECT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate
WHERE source_issue_id=$1 AND (proposal_origin='automation' OR approval_origin='automation' OR decision_origin='automation'
    OR sources @> '[{"kind":"delivery"}]'::jsonb OR evidence @> '[{"source":{"kind":"delivery"}}]'::jsonb))::boolean;

-- name: KnowledgeLineageRequiresV2 :one
SELECT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate
WHERE lineage_id=$1 AND (proposal_origin='automation' OR approval_origin='automation' OR decision_origin='automation'
    OR sources @> '[{"kind":"delivery"}]'::jsonb OR evidence @> '[{"source":{"kind":"delivery"}}]'::jsonb))::boolean;

-- name: KnowledgeTargetRequiresV2 :one
SELECT EXISTS (SELECT 1 FROM collaboration_knowledge_candidate
WHERE target_kind=$1 AND target_workspace_id=$2 AND target_project_id IS NOT DISTINCT FROM sqlc.narg(project_id)::uuid
    AND (proposal_origin='automation' OR approval_origin='automation' OR decision_origin='automation'
    OR sources @> '[{"kind":"delivery"}]'::jsonb OR evidence @> '[{"source":{"kind":"delivery"}}]'::jsonb))::boolean;
