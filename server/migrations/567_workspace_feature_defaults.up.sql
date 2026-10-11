-- Preserve saved workspace choices, including explicit opt-outs.
ALTER TABLE workspace_triage_settings ALTER COLUMN enabled SET DEFAULT true;
ALTER TABLE workspace_iteration_settings ALTER COLUMN enabled SET DEFAULT true;
