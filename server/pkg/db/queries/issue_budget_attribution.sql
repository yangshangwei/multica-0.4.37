-- name: GetLatestIssueBudgetAttemptForTask :one
SELECT * FROM issue_budget_attempt
WHERE workspace_id = @workspace_id AND task_id = @task_id
ORDER BY claim_generation DESC
LIMIT 1;
