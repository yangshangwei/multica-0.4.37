CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_decision_candidate_idx ON collaboration_stage_decision (candidate_id) WHERE step = 'proposal';
