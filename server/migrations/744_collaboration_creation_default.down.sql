-- Restores only the old default; the stored choice may be the administrator's.
ALTER TABLE collaboration_execution_gate ALTER COLUMN creation_enabled SET DEFAULT false;
