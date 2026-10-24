CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_resource_offer_issue_history ON collaboration_resource_offer(issue_id, created_at DESC, id DESC);
