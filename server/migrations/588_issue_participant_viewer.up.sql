CREATE INDEX CONCURRENTLY issue_participant_viewer_idx ON issue_participant (user_id, revoked_at, issue_id);
