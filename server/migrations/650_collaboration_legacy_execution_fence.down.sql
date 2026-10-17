-- Only an empty maintenance database may lose the collaboration execution
-- fence. Stop all API/background writers throughout a multi-file rollback.
DO $guard$
BEGIN
    LOCK TABLE agent_task_queue, comment, issue, issue_collaboration,
        issue_source_context IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM issue_collaboration) THEN
        RAISE EXCEPTION 'cannot remove the execution fence for retained collaboration issues; deploy a forward fix';
    END IF;
    EXECUTE $restore$
CREATE OR REPLACE FUNCTION lock_issue_execution(
    p_issue_id uuid,
    p_trigger_id uuid DEFAULT NULL,
    p_comment_ids uuid[] DEFAULT '{}',
    p_context jsonb DEFAULT NULL
) RETURNS boolean LANGUAGE plpgsql AS $body$
DECLARE
    issue_ids uuid[] := '{}';
    owner_id uuid;
    current_admission text;
    source_context_id uuid;
BEGIN
    IF p_issue_id IS NOT NULL THEN issue_ids := array_append(issue_ids, p_issue_id); END IF;
    IF p_context->>'type' = 'quick_create' THEN
        BEGIN
            owner_id := NULLIF(p_context->>'parent_issue_id', '')::uuid;
        EXCEPTION WHEN invalid_text_representation THEN owner_id := NULL;
        END;
        IF owner_id IS NOT NULL AND EXISTS(SELECT 1 FROM issue WHERE id = owner_id) THEN
            issue_ids := array_append(issue_ids, owner_id);
        END IF;
        BEGIN
            source_context_id := NULLIF(p_context->>'source_context_id', '')::uuid;
        EXCEPTION WHEN invalid_text_representation THEN source_context_id := NULL;
        END;
        IF source_context_id IS NOT NULL THEN
            SELECT source_issue_id INTO owner_id FROM issue_source_context WHERE id = source_context_id;
            IF owner_id IS NOT NULL AND EXISTS(SELECT 1 FROM issue WHERE id = owner_id) THEN
                issue_ids := array_append(issue_ids, owner_id);
            END IF;
        END IF;
    END IF;
    FOR owner_id IN SELECT DISTINCT id FROM unnest(issue_ids) id ORDER BY id LOOP
        SELECT admission_status INTO current_admission FROM issue WHERE id = owner_id FOR KEY SHARE NOWAIT;
        IF NOT FOUND OR current_admission NOT IN ('not_required', 'accepted') THEN RETURN false; END IF;
    END LOOP;
    IF EXISTS (
        SELECT 1 FROM comment
        WHERE id = ANY(array_append(COALESCE(p_comment_ids, '{}'), p_trigger_id))
          AND NOT dispatch_eligible
    ) THEN RETURN false; END IF;
    RETURN true;
EXCEPTION WHEN lock_not_available THEN
    RETURN false;
END;
$body$;
$restore$;
END;
$guard$;
