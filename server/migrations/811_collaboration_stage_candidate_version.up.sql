CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_candidate_version_idx ON collaboration_stage_candidate (issue_id, proposal_id, version);
