CREATE TABLE inbox_snooze (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    recipient_type text NOT NULL,
    recipient_id uuid NOT NULL,
    group_kind text NOT NULL CHECK (group_kind IN ('issue', 'item')),
    group_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 0,
    snoozed_until timestamptz,
    snoozed_at timestamptz,
    issue_revision_watermark bigint,
    reminded_at timestamptz,
    reminder_reason text,
    undo_token uuid,
    undo_snapshot jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE inbox_critical_source (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    recipient_type text NOT NULL,
    recipient_id uuid NOT NULL,
    group_kind text NOT NULL CHECK (group_kind IN ('issue', 'item')),
    group_id uuid NOT NULL,
    notification_type text NOT NULL,
    source_kind text NOT NULL,
    source_id uuid NOT NULL,
    source_revision bigint NOT NULL DEFAULT 0,
    inbox_item_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
