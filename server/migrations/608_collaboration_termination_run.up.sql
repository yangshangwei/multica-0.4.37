CREATE UNIQUE INDEX CONCURRENTLY collaboration_termination_run_idx ON collaboration_termination (run_id, fence, launch_nonce);
