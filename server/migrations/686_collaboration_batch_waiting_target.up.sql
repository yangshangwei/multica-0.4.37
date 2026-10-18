CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_batch_waiting_target ON collaboration_batch(channel_id,effective_target_id) WHERE state='waiting';
