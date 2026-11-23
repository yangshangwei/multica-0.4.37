-- NULL preserves legacy credentials. New protected role versions require a
-- strictly validated snapshot; clients cannot populate this field.
ALTER TABLE task_token ADD COLUMN api_capability jsonb;
