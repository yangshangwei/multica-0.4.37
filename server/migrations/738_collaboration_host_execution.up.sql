BEGIN;

ALTER TABLE collaboration_attempt
	ALTER COLUMN authorized_at DROP NOT NULL,
    ADD COLUMN launch_descriptor jsonb CHECK (launch_descriptor IS NULL OR
        (jsonb_typeof(launch_descriptor) = 'object' AND pg_column_size(launch_descriptor) <= 2048)),
    ADD COLUMN terminal_observation jsonb CHECK (terminal_observation IS NULL OR
        (jsonb_typeof(terminal_observation) = 'object' AND pg_column_size(terminal_observation) <= 8192));
ALTER TABLE collaboration_attempt ADD CONSTRAINT collaboration_host_attempt_authority_check CHECK (
    containment <> 'host_process_supervision_v1' OR (authorized_at IS NULL) = (launch_descriptor IS NULL));

ALTER TABLE collaboration_run DROP CONSTRAINT collaboration_run_proof_state_check;
ALTER TABLE collaboration_run ADD CONSTRAINT collaboration_run_proof_state_check
    CHECK (proof_state IN ('unknown', 'never_launched_proven', 'never_started_proven', 'exited_proven', 'host_exit_observed'));
ALTER TABLE collaboration_run ADD CONSTRAINT collaboration_run_host_proof_mode_check CHECK (
    (proof_state <> 'host_exit_observed' OR containment IS NOT DISTINCT FROM 'host_process_supervision_v1')
    AND NOT (containment IS NOT DISTINCT FROM 'host_process_supervision_v1' AND proof_state = 'exited_proven'));

-- A separate default-off admission gate allows new readers/receipts to ship
-- before any host runtime is admitted. Closing it never deletes old evidence.
ALTER TABLE collaboration_execution_gate ADD COLUMN host_execution_enabled boolean NOT NULL DEFAULT false;

CREATE FUNCTION collaboration_preserve_host_execution_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.containment IS NOT NULL AND NEW.containment IS DISTINCT FROM OLD.containment THEN
        RAISE EXCEPTION 'collaboration execution mode is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_run_host_mode BEFORE UPDATE ON collaboration_run
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_host_execution_identity();

CREATE FUNCTION collaboration_preserve_launch_observation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.run_id, NEW.attempt_seq, NEW.attempt_nonce, NEW.claim_generation, NEW.containment)
        IS DISTINCT FROM ROW(OLD.run_id, OLD.attempt_seq, OLD.attempt_nonce, OLD.claim_generation, OLD.containment)
        OR NEW.authorized_at IS DISTINCT FROM OLD.authorized_at
        OR NEW.launch_descriptor IS DISTINCT FROM OLD.launch_descriptor
        OR (OLD.terminal_observation IS NOT NULL AND NEW.terminal_observation IS DISTINCT FROM OLD.terminal_observation) THEN
        RAISE EXCEPTION 'collaboration launch identity and accepted observation are immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER collaboration_attempt_identity BEFORE UPDATE ON collaboration_attempt
    FOR EACH ROW EXECUTE FUNCTION collaboration_preserve_launch_observation();

-- Keep SQL resource release and group settlement on the same mode-aware
-- predicate. Go validates the matching retained-ledger facts before writes.
CREATE FUNCTION collaboration_managed_execution_ended(
    mode text, proof text, progress text, ended_at timestamptz,
    authorized bigint, sealed bigint, started_at timestamptz, outcome text
) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
    SELECT COALESCE(ended_at IS NOT NULL AND progress = 'settled' AND sealed >= authorized AND
        (mode IS NULL OR mode IN ('linux_pid_namespace_v1', 'host_process_supervision_v1')) AND (
            (proof = 'never_launched_proven' AND started_at IS NULL AND outcome IS DISTINCT FROM 'success') OR
            (proof = 'exited_proven' AND mode IS DISTINCT FROM 'host_process_supervision_v1') OR
            (proof = 'host_exit_observed' AND mode = 'host_process_supervision_v1' AND authorized > 0 AND sealed = authorized)
        ), false)
$$;

COMMIT;
