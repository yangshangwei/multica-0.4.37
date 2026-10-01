CREATE INDEX CONCURRENTLY admin_alert_latest_episode_idx ON admin_alert (organization_id,fingerprint,first_seen_at DESC,id DESC);
