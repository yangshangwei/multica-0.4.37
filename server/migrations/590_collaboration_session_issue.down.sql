DO $$
BEGIN
    LOCK TABLE collaboration_account_fence, collaboration_attachment, collaboration_attachment_reference, collaboration_command, collaboration_event, collaboration_execution_gate, collaboration_grant, collaboration_notification, collaboration_operation, collaboration_run, collaboration_run_event, collaboration_runtime_challenge, collaboration_runtime_enrollment, collaboration_session, collaboration_termination, comment, issue_collaboration, issue_handoff, issue_participant IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM collaboration_account_fence)
        OR EXISTS (SELECT 1 FROM collaboration_attachment)
        OR EXISTS (SELECT 1 FROM collaboration_attachment_reference)
        OR EXISTS (SELECT 1 FROM collaboration_command)
        OR EXISTS (SELECT 1 FROM collaboration_event)
        OR EXISTS (SELECT 1 FROM collaboration_execution_gate WHERE enabled OR creation_enabled OR epoch > 1)
        OR EXISTS (SELECT 1 FROM collaboration_grant)
        OR EXISTS (SELECT 1 FROM collaboration_notification)
        OR EXISTS (SELECT 1 FROM collaboration_operation)
        OR EXISTS (SELECT 1 FROM collaboration_run)
        OR EXISTS (SELECT 1 FROM collaboration_run_event)
        OR EXISTS (SELECT 1 FROM collaboration_runtime_challenge)
        OR EXISTS (SELECT 1 FROM collaboration_runtime_enrollment)
        OR EXISTS (SELECT 1 FROM collaboration_session)
        OR EXISTS (SELECT 1 FROM collaboration_termination)
        OR EXISTS (SELECT 1 FROM issue_collaboration)
        OR EXISTS (SELECT 1 FROM issue_handoff)
        OR EXISTS (SELECT 1 FROM issue_participant) THEN
        RAISE EXCEPTION 'refusing rollback: retained collaboration state exists';
    END IF;
    DROP INDEX collaboration_session_issue_idx;
END $$;
