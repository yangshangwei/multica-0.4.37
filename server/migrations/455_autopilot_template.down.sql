ALTER TABLE autopilot
    DROP COLUMN IF EXISTS template_key,
    DROP COLUMN IF EXISTS template_version;
