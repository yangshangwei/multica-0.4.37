ALTER TABLE report_snapshot ADD COLUMN seal_digest text CHECK (seal_digest IS NULL OR seal_digest ~ '^[0-9a-f]{64}$');
