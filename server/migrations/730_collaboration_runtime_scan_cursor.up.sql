-- Scheduling positions carry no execution authority or successful outcome.
-- Per-tier positions survive process replacement and are shared by pollers.
CREATE TABLE collaboration_runtime_scan_cursor (
    runtime_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    scan_kind text NOT NULL CHECK (scan_kind IN (
        'waiting', 'wake', 'reserved', 'reserved_continuation',
        'unreserved', 'unreserved_continuation'
    )),
    after_issue_id uuid,
    after_created_at timestamptz,
    after_id uuid,
    CHECK (
        (after_id IS NULL AND after_issue_id IS NULL AND after_created_at IS NULL)
        OR (after_id IS NOT NULL AND (
            (scan_kind = 'waiting' AND after_issue_id IS NOT NULL AND after_created_at IS NULL)
            OR (scan_kind <> 'waiting' AND after_issue_id IS NULL AND after_created_at IS NOT NULL)
        ))
    )
);
