CREATE INDEX CONCURRENTLY managed_installation_scope_page_idx ON managed_installation (organization_id, lifecycle, created_at DESC, id DESC);
