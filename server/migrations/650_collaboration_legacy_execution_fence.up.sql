-- Collaboration sources must never enter ordinary execution. R39 creates the
-- issue and its collaboration identity atomically; no ordinary conversion is
-- supported. Keep the existing triage locks and pending-comment eligibility.
-- A parent reference alone is not a content source: R51 ordinary children and
-- quick-create parent links retain their ordinary execution contract.
CREATE OR REPLACE FUNCTION lock_issue_execution(
    p_issue_id uuid,
    p_trigger_id uuid DEFAULT NULL,
    p_comment_ids uuid[] DEFAULT '{}',
    p_context jsonb DEFAULT NULL
) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE
    issue_ids uuid[] := '{}';
    owner_id uuid;
    current_admission text;
    source_context_id uuid;
    captured_source_id uuid;
BEGIN
    IF p_issue_id IS NOT NULL THEN issue_ids := array_append(issue_ids, p_issue_id); END IF;
    IF p_context->>'type' = 'quick_create' THEN
        -- Deleted ordinary sources retain immutable snapshots. Malformed or
        -- missing context still belongs to the existing claim validation.
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
            SELECT source_issue_id INTO captured_source_id FROM issue_source_context WHERE id = source_context_id;
            IF captured_source_id IS NOT NULL AND EXISTS(SELECT 1 FROM issue WHERE id = captured_source_id) THEN
                issue_ids := array_append(issue_ids, captured_source_id);
            END IF;
        END IF;
    END IF;
    FOR owner_id IN SELECT DISTINCT id FROM unnest(issue_ids) id ORDER BY id LOOP
        SELECT admission_status INTO current_admission FROM issue WHERE id = owner_id FOR KEY SHARE NOWAIT;
        IF NOT FOUND OR current_admission NOT IN ('not_required', 'accepted') THEN RETURN false; END IF;
    END LOOP;
    IF EXISTS (
        SELECT 1 FROM issue_collaboration
        WHERE issue_id = p_issue_id OR issue_id = captured_source_id
    ) THEN RETURN false; END IF;
    IF EXISTS (
        SELECT 1 FROM comment c
        WHERE c.id = ANY(array_append(COALESCE(p_comment_ids, '{}'), p_trigger_id))
          AND (NOT c.dispatch_eligible OR EXISTS (
              SELECT 1 FROM issue_collaboration WHERE issue_id = c.issue_id
          ))
    ) THEN RETURN false; END IF;
    RETURN true;
EXCEPTION WHEN lock_not_available THEN
    RETURN false;
END;
$$;
