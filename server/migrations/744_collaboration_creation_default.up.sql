BEGIN;

-- New installations allow collaboration creation; execution, host admission
-- and the epoch keep their own defaults. ALTER takes the table lock first, so a
-- console change still in flight commits its receipt before the check below.
ALTER TABLE collaboration_execution_gate ALTER COLUMN creation_enabled SET DEFAULT true;

-- Upgrades open the old default only. A successful console open or close is
-- the administrator's choice and keeps the stored value; refused receipts and
-- execution-only changes say nothing about creation.
UPDATE collaboration_execution_gate SET creation_enabled = true
WHERE id = 1 AND NOT creation_enabled AND NOT EXISTS (
    SELECT 1 FROM admin_operation
    WHERE kind = 'collaboration.gate' AND target_kind = 'collaboration_gate' AND state = 'succeeded'
        AND result_code IN ('collaboration_creation_opened', 'collaboration_creation_closed'));

COMMIT;
