ALTER TABLE issue ADD COLUMN IF NOT EXISTS admission_status text NOT NULL DEFAULT 'not_required' CHECK (admission_status IN ('not_required','pending','accepted','rejected','duplicate'));
CREATE TABLE workspace_triage_settings (
 workspace_id uuid NOT NULL, enabled boolean NOT NULL DEFAULT false,
 acceptance_status text NOT NULL DEFAULT 'todo', require_priority boolean NOT NULL DEFAULT false,
 responsibility_mode text NOT NULL DEFAULT 'none' CHECK (responsibility_mode IN ('none','notify','assign')),
 responsibility_member_id uuid, revision bigint NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by uuid
);
CREATE TABLE issue_triage (
 issue_id uuid NOT NULL, workspace_id uuid NOT NULL, round integer NOT NULL DEFAULT 1,
 first_entered_at timestamptz NOT NULL DEFAULT now(), entered_at timestamptz NOT NULL DEFAULT now(),
 reviewer_id uuid, snoozed_until timestamptz, candidate_project_id uuid,
 candidate_assignee_type text, candidate_assignee_id uuid,
 source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','csv')), source_url text,
 external_id text, batch_id uuid, filename text, row_number integer,
 duplicate_issue_id uuid, duplicate_identifier text
);
CREATE TABLE triage_intake_request (
 workspace_id uuid NOT NULL, actor_id uuid NOT NULL, request_id uuid NOT NULL,
 payload_hash text NOT NULL, issue_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE triage_action (
 id uuid NOT NULL, workspace_id uuid NOT NULL, issue_id uuid NOT NULL, actor_id uuid NOT NULL,
 request_id uuid NOT NULL, payload_hash text NOT NULL, action text NOT NULL, round integer NOT NULL,
 reason text, before_snapshot jsonb NOT NULL, after_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), execution_status text NOT NULL DEFAULT 'not_requested',
 task_id uuid, execution_error text, execution_context jsonb NOT NULL DEFAULT '{}'
);
CREATE TABLE triage_import_batch (
 id uuid NOT NULL, workspace_id uuid NOT NULL, actor_id uuid NOT NULL, request_id uuid NOT NULL,
 payload_hash text NOT NULL, filename text NOT NULL, headers jsonb NOT NULL DEFAULT '[]',
 mapping jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now(),
 committed_at timestamptz, created_count integer NOT NULL DEFAULT 0,
 skipped_count integer NOT NULL DEFAULT 0, failed_count integer NOT NULL DEFAULT 0
);
CREATE TABLE triage_import_row (
 batch_id uuid NOT NULL, workspace_id uuid NOT NULL, row_number integer NOT NULL,
 values jsonb NOT NULL DEFAULT '{}', normalized jsonb NOT NULL DEFAULT '{}',
 warnings jsonb NOT NULL DEFAULT '[]', errors jsonb NOT NULL DEFAULT '[]',
 duplicate boolean NOT NULL DEFAULT false, duplicate_issue_id uuid, similar_issue_ids jsonb NOT NULL DEFAULT '[]',
 external_id text, status text NOT NULL DEFAULT 'ready', issue_id uuid, error text,
 import_duplicate boolean NOT NULL DEFAULT false, attempted_at timestamptz
);
CREATE TABLE triage_notification (
 id uuid NOT NULL, workspace_id uuid NOT NULL, recipient_id uuid NOT NULL, event_key text NOT NULL,
 issue_id uuid, batch_id uuid, title text NOT NULL, details jsonb NOT NULL DEFAULT '{}',
 due_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
