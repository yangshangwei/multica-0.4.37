CREATE INDEX CONCURRENTLY collaboration_event_outbox_idx ON collaboration_event (created_at, id) WHERE outbox_state = 'pending';
