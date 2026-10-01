CREATE INDEX CONCURRENTLY admin_alert_condition_recovery_idx ON admin_alert (organization_id,rule,condition_active,first_seen_at,id);
