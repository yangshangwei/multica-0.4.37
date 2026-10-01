CREATE INDEX CONCURRENTLY admin_alert_status_page_idx ON admin_alert (organization_id,status,first_seen_at DESC,id DESC);
