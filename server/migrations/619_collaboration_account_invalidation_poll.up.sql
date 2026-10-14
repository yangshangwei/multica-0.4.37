CREATE INDEX CONCURRENTLY idx_collaboration_account_invalidation_poll
ON collaboration_account_invalidation ((COALESCE(last_attempted_at, created_at)), user_id, account_epoch)
WHERE reconciled_at IS NULL;
