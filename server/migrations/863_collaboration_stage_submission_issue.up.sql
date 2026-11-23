CREATE INDEX CONCURRENTLY collaboration_stage_submission_issue_idx ON collaboration_stage_submission (issue_id, lifecycle_epoch, created_at, submission_id);
