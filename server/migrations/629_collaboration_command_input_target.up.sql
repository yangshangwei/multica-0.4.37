CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_command_input_target ON collaboration_command (input_event_id, target_id);
