CREATE INDEX CONCURRENTLY idx_collaboration_account_invalidation_pending
ON collaboration_account_invalidation (created_at, user_id, account_epoch)
WHERE reconciled_at IS NULL;
