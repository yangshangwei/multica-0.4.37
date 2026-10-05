ALTER TABLE project
    ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    ADD COLUMN IF NOT EXISTS description_revision bigint NOT NULL DEFAULT 1 CHECK (description_revision > 0),
    ADD COLUMN IF NOT EXISTS in_progress_since timestamptz,
    ADD COLUMN IF NOT EXISTS in_progress_since_source text CHECK (in_progress_since_source IN ('transition', 'migration'));
ALTER TABLE workspace ADD COLUMN IF NOT EXISTS planning_timezone text;
UPDATE project SET in_progress_since = now(), in_progress_since_source = 'migration'
WHERE status = 'in_progress' AND in_progress_since IS NULL;

CREATE TABLE IF NOT EXISTS project_state_change (
 id uuid NOT NULL DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 actor_type text NOT NULL CHECK (actor_type IN ('member','agent')), actor_id uuid NOT NULL,
 from_status text NOT NULL, to_status text NOT NULL, reason text,
 created_at timestamptz NOT NULL DEFAULT now(), project_revision bigint NOT NULL CHECK (project_revision > 0)
);
CREATE TABLE IF NOT EXISTS project_update (
 id uuid NOT NULL DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 author_user_id uuid NOT NULL, published_at timestamptz NOT NULL DEFAULT now(),
 current_revision bigint NOT NULL DEFAULT 1 CHECK (current_revision > 0)
);
CREATE TABLE IF NOT EXISTS project_update_revision (
 id uuid NOT NULL DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 update_id uuid NOT NULL, revision bigint NOT NULL CHECK (revision > 0), editor_user_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), kind text NOT NULL CHECK (kind IN ('progress','risk','acceptance')),
 body text NOT NULL, health_judgment text CHECK (health_judgment IN ('on_track','attention','risk')),
 correction_reason text, evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
 statistics_snapshot jsonb, acceptance jsonb,
 CHECK ((kind = 'acceptance') = (acceptance IS NOT NULL)),
 CHECK (revision = 1 OR (correction_reason IS NOT NULL AND length(trim(correction_reason)) > 0))
);
CREATE TABLE IF NOT EXISTS project_update_request (
 workspace_id uuid NOT NULL, project_id uuid NOT NULL, actor_user_id uuid NOT NULL,
 request_id uuid NOT NULL, operation text NOT NULL CHECK (operation IN ('create','correct')),
 payload_hash text NOT NULL, update_id uuid NOT NULL, result_revision bigint NOT NULL CHECK (result_revision > 0),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS project_update_notification (
 id uuid NOT NULL DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL, project_id uuid NOT NULL,
 update_id uuid NOT NULL, recipient_user_id uuid NOT NULL, source_revision bigint NOT NULL CHECK (source_revision > 0),
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','delivered','cancelled','dead_letter')),
 delivered_at timestamptz, attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 next_attempt_at timestamptz, last_error_code text
);
