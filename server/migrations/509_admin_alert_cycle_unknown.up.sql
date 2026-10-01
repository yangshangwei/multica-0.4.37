ALTER TABLE admin_alert_detector_state ADD COLUMN cycle_unknown_count bigint NOT NULL DEFAULT 0 CHECK (cycle_unknown_count >= 0);
