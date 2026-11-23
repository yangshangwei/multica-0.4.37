CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_switch_consent_resource ON collaboration_switch_consent(proposal_id, agent_id);
