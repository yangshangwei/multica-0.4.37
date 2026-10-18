CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_switch_proposal_target ON collaboration_switch_proposal(target_id, created_at, id);
