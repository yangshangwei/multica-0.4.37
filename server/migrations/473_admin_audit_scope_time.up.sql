CREATE INDEX CONCURRENTLY admin_audit_scope_time_idx ON admin_audit_event (organization_id, created_at DESC, id DESC);
