CREATE INDEX CONCURRENTLY collaboration_attachment_draft_idx ON collaboration_attachment (draft_id, uploader_user_id) WHERE issue_id IS NULL;
