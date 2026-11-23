BEGIN;

-- NULL is executable intent, not a fallback for unknown authority. Existing
-- active targets keep their intent; channel intent and proof rows stay intact.
ALTER TABLE collaboration_target
    ADD COLUMN IF NOT EXISTS hold_reason text,
    ADD COLUMN IF NOT EXISTS hold_version bigint;

UPDATE collaboration_target
SET hold_reason = CASE
        WHEN revoked_at IS NOT NULL OR pause_reason IN
            ('owner_revoked', 'participant_removed', 'participant_left', 'credentials_revoked', 'grant_required')
            THEN 'grant_required'
        WHEN pause_reason = 'configuration_changed' THEN 'switch_hold'
        WHEN pause_reason IN ('config_invalid', 'target_configuration_changed') THEN 'config_invalid'
        WHEN queue_state = 'paused' THEN 'initial'
        ELSE NULL
    END,
    hold_version = 1
WHERE hold_version IS NULL;

-- Before target holds, Reconfigure advanced target/config/control together but
-- left the channel unchanged. Repair only that recognizable local-control
-- footprint, including repeated reconfigures since the last channel mirror.
-- A control delta larger than its configuration history, an unaccounted group
-- generation, or any unresolved/contradictory retained proof is not repairable.
-- Hold reason/version and all channel, input, group and run rows are preserved.
UPDATE collaboration_target AS t
SET queue_state = c.queue_state, pause_reason = c.pause_reason, control_version = c.control_version
FROM collaboration_channel AS c
WHERE c.id = t.channel_id AND c.session_id = t.session_id
  AND t.hold_reason = 'switch_hold' AND t.queue_state = 'paused' AND t.pause_reason = 'configuration_changed'
  AND t.target_epoch = t.config_version AND t.config_version > 1
  AND t.control_version > c.control_version
  AND t.control_version - c.control_version <= t.config_version - 1
  AND t.group_generation = c.group_generation AND t.active_group_id IS NULL AND c.active_group_id IS NULL
  AND (c.group_generation = 0 OR EXISTS (
      SELECT 1 FROM collaboration_execution_group g
      WHERE g.session_id = c.session_id AND g.channel_id = c.id AND g.generation = c.group_generation AND g.settled_at IS NOT NULL
  ))
  AND NOT EXISTS (
      SELECT 1 FROM collaboration_execution_group g
      WHERE (g.target_id = t.id OR g.channel_id = c.id)
        AND (g.session_id <> t.session_id OR g.channel_id IS DISTINCT FROM c.id OR g.batch_id IS NULL
          OR g.settled_at IS NULL OR g.expansion_sealed_at IS NULL OR g.generation > c.group_generation
          OR (g.target_id = t.id AND (g.target_epoch >= t.target_epoch OR g.config_version >= t.config_version))
          OR NOT EXISTS (SELECT 1 FROM collaboration_run r WHERE r.group_id = g.id))
  )
  AND NOT EXISTS (
      SELECT 1 FROM collaboration_run r LEFT JOIN collaboration_execution_group g ON g.id = r.group_id
      WHERE (r.target_id = t.id OR r.channel_id = c.id)
        AND (r.settled_at IS NULL OR g.id IS NULL OR r.session_id <> g.session_id
          OR r.target_id IS DISTINCT FROM g.target_id OR r.command_id IS DISTINCT FROM g.command_id
          OR r.channel_id IS DISTINCT FROM g.channel_id OR r.batch_id IS DISTINCT FROM g.batch_id
          OR r.target_epoch IS DISTINCT FROM g.target_epoch OR r.config_version IS DISTINCT FROM g.config_version
          OR r.proof_state NOT IN ('never_launched_proven', 'exited_proven') OR r.sealed_attempt_seq IS NULL
          OR r.sealed_attempt_seq < r.authorized_attempt_seq
          OR (r.proof_state = 'never_launched_proven' AND (r.authorized_attempt_seq <> 0 OR r.started_at IS NOT NULL))
          OR (r.parent_run_id IS NOT NULL AND NOT EXISTS (
              SELECT 1 FROM collaboration_run parent WHERE parent.id = r.parent_run_id AND parent.group_id = r.group_id
          )))
  )
  AND NOT EXISTS (
      SELECT 1 FROM collaboration_delegation d
      JOIN collaboration_execution_group g ON g.id = d.group_id
      LEFT JOIN collaboration_run parent ON parent.id = d.parent_run_id AND parent.group_id = g.id
      WHERE (g.target_id = t.id OR g.channel_id = c.id)
        AND (d.state NOT IN ('settled', 'cancelled') OR parent.id IS NULL)
  );

ALTER TABLE collaboration_target
    ALTER COLUMN hold_reason SET DEFAULT 'initial',
    ALTER COLUMN hold_version SET DEFAULT 1,
    ALTER COLUMN hold_version SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
        WHERE conrelid = 'collaboration_target'::regclass AND conname = 'collaboration_target_hold_reason_check') THEN
        ALTER TABLE collaboration_target ADD CONSTRAINT collaboration_target_hold_reason_check
            CHECK (hold_reason IN ('initial', 'grant_required', 'config_invalid', 'switch_hold'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
        WHERE conrelid = 'collaboration_target'::regclass AND conname = 'collaboration_target_hold_version_check') THEN
        ALTER TABLE collaboration_target ADD CONSTRAINT collaboration_target_hold_version_check CHECK (hold_version > 0);
    END IF;
END;
$$;

COMMIT;
