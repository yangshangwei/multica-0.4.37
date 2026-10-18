CREATE INDEX CONCURRENTLY idx_collaboration_batch_waiting ON collaboration_batch(channel_id,seq) WHERE state='waiting';
