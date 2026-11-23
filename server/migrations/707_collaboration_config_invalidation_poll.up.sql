CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_config_invalidation_poll ON collaboration_config_invalidation((COALESCE(last_attempted_at, created_at)), id) WHERE reconciled_at IS NULL;
