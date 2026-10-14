CREATE UNIQUE INDEX CONCURRENTLY collaboration_run_unsettled_idx ON collaboration_run (session_id) WHERE settled_at IS NULL;
