ALTER TABLE project_delivery_policy DROP CONSTRAINT IF EXISTS project_delivery_policy_mode_check;
ALTER TABLE project_delivery_policy ADD CONSTRAINT project_delivery_policy_mode_check CHECK (mode IN ('off','observe','enforce'));
