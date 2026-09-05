ALTER TABLE agent
    DROP COLUMN IF EXISTS template_key,
    DROP COLUMN IF EXISTS template_version,
    DROP COLUMN IF EXISTS autonomy_level;
