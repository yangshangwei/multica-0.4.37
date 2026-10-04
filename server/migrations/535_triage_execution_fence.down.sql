DO $$ BEGIN
    LOCK TABLE comment, issue, issue_triage, triage_action, triage_import_batch,
        triage_import_row, triage_intake_request, triage_notification,
        workspace_triage_settings IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM comment WHERE NOT dispatch_eligible)
        OR EXISTS (SELECT 1 FROM issue WHERE admission_status <> 'not_required')
        OR EXISTS (SELECT 1 FROM issue_triage)
        OR EXISTS (SELECT 1 FROM triage_action)
        OR EXISTS (SELECT 1 FROM triage_import_batch)
        OR EXISTS (SELECT 1 FROM triage_import_row)
        OR EXISTS (SELECT 1 FROM triage_intake_request)
        OR EXISTS (SELECT 1 FROM triage_notification)
        OR EXISTS (SELECT 1 FROM workspace_triage_settings) THEN
        RAISE EXCEPTION 'cannot remove triage comment admission history; deploy a forward fix';
    END IF;
    DROP FUNCTION lock_issue_execution(uuid, uuid, uuid[], jsonb);
    ALTER TABLE comment DROP COLUMN dispatch_eligible;
END $$;
