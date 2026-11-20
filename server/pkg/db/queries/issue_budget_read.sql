-- name: ListIssueBudgetAttemptPage :many
-- The strict tuple boundary also serves the first page, using +infinity.
SELECT * FROM issue_budget_attempt
WHERE workspace_id = @workspace_id AND budget_root_id = @budget_root_id
  AND (created_at, id) < (@before_created_at::timestamptz, @before_id::uuid)
ORDER BY created_at DESC, id DESC
LIMIT @page_size::integer;

-- name: ListIssueBudgetObservationPage :many
-- Coverage uses the same task/generation test as CountIssueBudgetUncoveredGaps.
-- Only in-flight coverage gaps can be covered; retained rows are never changed.
SELECT sqlc.embed(o),
       (o.kind = 'coverage_gap'
        AND o.reason IN ('in_flight_legacy', 'in_flight_at_bind')
        AND o.task_id IS NOT NULL
        AND o.claim_generation IS NOT NULL
        AND EXISTS (
            SELECT 1 FROM issue_budget_attempt a
            WHERE a.task_id = o.task_id AND a.claim_generation = o.claim_generation
              AND a.workspace_id = o.workspace_id
        ))::boolean AS covered
FROM issue_budget_observation o
WHERE o.workspace_id = @workspace_id AND o.budget_root_id = @budget_root_id
  AND (o.created_at, o.id) < (@before_created_at::timestamptz, @before_id::uuid)
ORDER BY o.created_at DESC, o.id DESC
LIMIT @page_size::integer;
