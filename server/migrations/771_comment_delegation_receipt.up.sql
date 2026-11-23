CREATE TABLE comment_delegation_receipt (
 workspace_id uuid NOT NULL, actor_type text NOT NULL CHECK (actor_type IN ('member','agent')),
 actor_id uuid NOT NULL, originator_id uuid NOT NULL, request_key uuid NOT NULL,
 request_digest text NOT NULL, issue_id uuid NOT NULL, comment_id uuid NOT NULL, source_task_id uuid,
 state text NOT NULL CHECK (state IN ('unknown','recorded')), targets jsonb NOT NULL, response jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), recorded_at timestamptz
);
