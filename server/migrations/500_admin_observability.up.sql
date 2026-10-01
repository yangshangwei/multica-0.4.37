ALTER TABLE agent_task_queue
    ADD COLUMN queued_at timestamptz,
    ADD COLUMN queued_at_source text CHECK (queued_at_source IN ('transition','observation')),
    ADD CONSTRAINT agent_task_queue_clock_consistent CHECK ((queued_at IS NULL) = (queued_at_source IS NULL));

UPDATE agent_task_queue SET queued_at=now(),queued_at_source='observation' WHERE status='queued';

CREATE FUNCTION multica_track_task_queue_time() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='INSERT' THEN
        IF NEW.status='queued' THEN
            NEW.queued_at := NEW.created_at;
            NEW.queued_at_source := 'transition';
        END IF;
    ELSIF NEW.status='queued' AND OLD.status IS DISTINCT FROM NEW.status THEN
        NEW.queued_at := now();
        NEW.queued_at_source := 'transition';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER agent_task_queue_clock
BEFORE INSERT OR UPDATE OF status ON agent_task_queue
FOR EACH ROW EXECUTE FUNCTION multica_track_task_queue_time();

ALTER TABLE admin_audit_event ADD COLUMN actor_display_name text;

CREATE TABLE admin_alert (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    rule text NOT NULL CHECK (rule IN ('installation_unreachable','queue_timeout','execution_failed')),
    subject_kind text NOT NULL CHECK (subject_kind IN ('installation','task')),
    subject_id uuid NOT NULL,
    fingerprint text NOT NULL CHECK (length(fingerprint) BETWEEN 1 AND 256),
    severity text NOT NULL CHECK (severity IN ('warning','critical')),
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved','closed')),
    condition_active boolean NOT NULL DEFAULT true,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    last_observed_at timestamptz NOT NULL DEFAULT now(),
    occurrence_count bigint NOT NULL DEFAULT 1 CHECK (occurrence_count > 0),
    assignee_id uuid,
    acknowledged_at timestamptz,
    resolved_at timestamptz,
    closed_at timestamptz,
    resolution_code text,
    related_task_id uuid,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    operation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_alert_detector_state (
    organization_id uuid NOT NULL,
    rule text NOT NULL CHECK (rule IN ('installation_unreachable','queue_timeout','execution_failed')),
    cursor_time timestamptz,
    cursor_id uuid,
    cycle_started_at timestamptz,
    last_started_at timestamptz,
    last_successful_at timestamptz,
    last_error_code text,
    source_state text NOT NULL DEFAULT 'unknown' CHECK (source_state IN ('unknown','healthy','unavailable')),
    scan_complete boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    lease_owner uuid,
    lease_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
