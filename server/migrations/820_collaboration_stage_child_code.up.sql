-- The explicit existing-child confirmation binds the code version it carried
-- forward: a server-verified pull request and head, or none. No index, no
-- foreign key; the pull request is resolved in the confirming transaction.
ALTER TABLE collaboration_stage_child
 ADD COLUMN code_provider text,
 ADD COLUMN code_pull_request_id uuid,
 ADD COLUMN code_head_sha text,
 ADD CONSTRAINT collaboration_stage_child_code CHECK (
  (code_provider IS NULL AND code_pull_request_id IS NULL AND code_head_sha IS NULL)
  OR (code_provider IN ('github','vcs') AND code_pull_request_id IS NOT NULL
      AND code_head_sha IS NOT NULL AND code_head_sha ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'));
