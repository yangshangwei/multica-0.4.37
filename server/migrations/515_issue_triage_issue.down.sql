DO $$ BEGIN
LOCK TABLE issue_triage, triage_action, triage_import_batch, triage_import_row, triage_intake_request, triage_notification, workspace_triage_settings, issue IN ACCESS EXCLUSIVE MODE NOWAIT;
IF EXISTS (SELECT 1 FROM workspace_triage_settings) OR EXISTS (SELECT 1 FROM triage_import_row) OR EXISTS (SELECT 1 FROM triage_notification) OR EXISTS (SELECT 1 FROM issue_triage) OR EXISTS (SELECT 1 FROM triage_action) OR EXISTS (SELECT 1 FROM triage_import_batch) OR EXISTS (SELECT 1 FROM triage_intake_request) OR EXISTS (SELECT 1 FROM issue WHERE admission_status <> 'not_required') THEN RAISE EXCEPTION 'triage data exists; downgrade refused'; END IF;
DROP INDEX IF EXISTS issue_triage_issue;
END $$;
