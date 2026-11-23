CREATE UNIQUE INDEX CONCURRENTLY collaboration_attachment_upload_request_idx ON collaboration_attachment (uploader_user_id, draft_id, upload_request_id) WHERE upload_request_id IS NOT NULL;
