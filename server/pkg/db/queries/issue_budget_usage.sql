-- name: GetIssueBudgetUsageAttempt :one
-- Entry-point lookup before workspace/runtime authorization. This deliberately
-- does not join the task or issue: their deletion cannot erase attribution.
SELECT * FROM issue_budget_attempt WHERE id = @id;

-- name: CountIssueBudgetUsageBatches :one
-- Under the root lock. The scope prefix contains attempt_open and at most
-- one usage_batch; the claim operation does not consume the usage slot.
SELECT count(*) FROM issue_budget_operation
WHERE workspace_id = @workspace_id AND scope_kind = 'attempt'
  AND scope_id = @scope_id AND operation_kind = 'usage_batch';
