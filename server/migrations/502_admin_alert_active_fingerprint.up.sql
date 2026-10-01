CREATE UNIQUE INDEX CONCURRENTLY admin_alert_active_fingerprint_uidx ON admin_alert (organization_id,fingerprint) WHERE condition_active;
