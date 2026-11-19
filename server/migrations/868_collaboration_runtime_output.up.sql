ALTER TABLE collaboration_attachment
    ALTER COLUMN uploader_user_id DROP NOT NULL,
    ADD COLUMN uploader_agent_id uuid,
    ADD COLUMN run_id uuid,
    ADD COLUMN attempt_seq bigint,
    DROP CONSTRAINT collaboration_attachment_delivery_context,
    ADD CONSTRAINT collaboration_attachment_uploader_identity CHECK (
        (uploader_user_id IS NOT NULL AND uploader_agent_id IS NULL
            AND run_id IS NULL AND attempt_seq IS NULL)
        OR (uploader_user_id IS NULL AND uploader_agent_id IS NOT NULL
            AND run_id IS NOT NULL AND attempt_seq IS NOT NULL AND attempt_seq > 0
            AND draft_id IS NULL AND intended_issue_id IS NULL AND upload_request_id IS NULL
            AND issue_id IS NOT NULL AND delivered_at IS NOT NULL
            AND encryption_key IS NOT NULL AND discarded_at IS NULL)
    ),
    ADD CONSTRAINT collaboration_attachment_delivery_context CHECK (
        encryption_key IS NULL OR (
            source_workspace_id IS NOT NULL
            AND content_sha256 IS NOT NULL AND content_sha256 ~ '^[0-9a-f]{64}$'
            AND size_bytes <= 104857600
            AND storage_key LIKE 'private/collaboration/%'
            AND (intended_issue_id IS NULL OR issue_id IS NULL OR intended_issue_id = issue_id)
            AND (uploader_agent_id IS NOT NULL OR (
                draft_id IS NOT NULL AND upload_request_id IS NOT NULL
                AND length(upload_request_id) BETWEEN 1 AND 128
            ))
        )
    );
