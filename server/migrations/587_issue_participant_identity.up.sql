CREATE UNIQUE INDEX CONCURRENTLY issue_participant_identity_idx ON issue_participant (issue_id, user_id);
