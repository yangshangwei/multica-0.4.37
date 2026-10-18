CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_collaboration_command_channel_sequence ON collaboration_command(channel_id,seq);
