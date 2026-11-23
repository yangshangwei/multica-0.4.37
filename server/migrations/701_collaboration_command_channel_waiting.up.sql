CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_command_channel_waiting ON collaboration_command(channel_id,state,seq);
