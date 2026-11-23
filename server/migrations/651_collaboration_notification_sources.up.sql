ALTER TABLE collaboration_notification
    ADD COLUMN source_kind text NOT NULL DEFAULT 'issue'
        CHECK (source_kind IN ('issue', 'comment', 'body')),
    ADD COLUMN source_id uuid;
