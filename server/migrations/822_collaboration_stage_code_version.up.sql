CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_code_version_idx ON collaboration_stage_candidate (issue_id, version) WHERE kind='code';
