CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_decision_accept_idx ON collaboration_stage_decision (proposal_id) WHERE step = 'proposal' AND decision = 'accepted';
