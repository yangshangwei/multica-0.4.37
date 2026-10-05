ALTER TABLE workspace_mcp_server ADD COLUMN IF NOT EXISTS template_source TEXT;

UPDATE workspace_mcp_server
SET template_source = 'builtin'
WHERE template_key IS NOT NULL AND template_source IS NULL;
