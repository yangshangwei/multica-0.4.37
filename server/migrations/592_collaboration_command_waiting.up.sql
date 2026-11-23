CREATE INDEX CONCURRENTLY collaboration_command_waiting_idx ON collaboration_command (session_id, state, seq);
