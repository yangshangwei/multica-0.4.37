ALTER TABLE collaboration_attachment
    ADD COLUMN source_workspace_id uuid,
    ADD COLUMN intended_issue_id uuid,
    ADD COLUMN upload_request_id text,
    ADD COLUMN content_sha256 text,
    ADD COLUMN encryption_key bytea,
    ADD COLUMN discarded_at timestamptz,
    ADD CONSTRAINT collaboration_attachment_private_key CHECK (
        encryption_key IS NULL OR octet_length(encryption_key) = 32
    ),
    ADD CONSTRAINT collaboration_attachment_delivery_context CHECK (
        encryption_key IS NULL OR (
            source_workspace_id IS NOT NULL AND draft_id IS NOT NULL
            AND upload_request_id IS NOT NULL AND length(upload_request_id) BETWEEN 1 AND 128
            AND content_sha256 IS NOT NULL AND content_sha256 ~ '^[0-9a-f]{64}$'
            AND size_bytes <= 104857600
            AND storage_key LIKE 'private/collaboration/%'
            AND (intended_issue_id IS NULL OR issue_id IS NULL OR intended_issue_id = issue_id)
        )
    ),
    ADD CONSTRAINT collaboration_attachment_discard_undelivered CHECK (
        discarded_at IS NULL OR (issue_id IS NULL AND delivered_at IS NULL)
    );
