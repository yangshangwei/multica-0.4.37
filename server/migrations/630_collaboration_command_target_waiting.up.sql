CREATE INDEX CONCURRENTLY idx_collaboration_command_target_waiting ON collaboration_command (target_id, state, seq);
