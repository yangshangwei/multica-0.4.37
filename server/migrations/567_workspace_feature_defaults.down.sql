-- Restore defaults without rewriting choices saved while the feature was enabled.
ALTER TABLE workspace_triage_settings ALTER COLUMN enabled SET DEFAULT false;
ALTER TABLE workspace_iteration_settings ALTER COLUMN enabled SET DEFAULT false;
