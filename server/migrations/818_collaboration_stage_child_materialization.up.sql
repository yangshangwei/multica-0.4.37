CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_child_materialization_idx ON collaboration_stage_child (proposal_id, ordinal) WHERE origin = 'materialized';
