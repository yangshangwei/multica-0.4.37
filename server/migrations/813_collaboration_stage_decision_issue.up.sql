CREATE INDEX CONCURRENTLY collaboration_stage_decision_issue_idx ON collaboration_stage_decision (issue_id, created_at, decision_id);
