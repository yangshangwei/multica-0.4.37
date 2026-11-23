CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_config_invalidation_resource ON collaboration_config_invalidation(resource_kind, resource_id, config_epoch, id) WHERE reconciled_at IS NULL;
