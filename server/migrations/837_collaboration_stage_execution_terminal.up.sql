CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_execution_terminal_idx ON collaboration_stage_execution_observation (intent_id) WHERE result IN ('succeeded','failed','not_started');
