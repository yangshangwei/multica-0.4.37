CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_run_leader_turn ON collaboration_run(group_id,leader_turn) WHERE leader_turn IS NOT NULL;
