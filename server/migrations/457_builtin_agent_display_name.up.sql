-- Change product-owned defaults without changing system identity or owner content.
-- One statement keeps the name, description and session-title updates atomic.
DO $migration$
DECLARE
    candidate RECORD;
    conflict_constraint TEXT;
    display_name CONSTANT TEXT := '小阿孚';
    old_description_en CONSTANT TEXT := 'Your workspace Chief of Staff. Mika turns goals into issues, coordinates agents, and helps build reusable workflows.';
    old_description_zh CONSTANT TEXT := '你的工作区 Chief of Staff。Mika 会把目标转化为任务、协调智能体，并帮你建立可复用的工作流。';
    new_description_en CONSTANT TEXT := 'Your workspace Chief of Staff. Turns goals into issues, coordinates agents, and helps build reusable workflows.';
    new_description_zh CONSTANT TEXT := '你的工作区 Chief of Staff。把目标转化为任务、协调智能体，并帮你建立可复用的工作流。';
BEGIN
    FOR candidate IN
        SELECT id, workspace_id
        FROM agent
        WHERE system_key = 'mika' AND name = 'Mika'
        ORDER BY id
    LOOP
        -- Archived agents still own their names under agent_workspace_name_unique.
        IF EXISTS (
            SELECT 1 FROM agent
            WHERE workspace_id = candidate.workspace_id
              AND name = display_name
              AND id <> candidate.id
        ) THEN
            RAISE NOTICE 'Skipped built-in agent rename: agent_id=%, workspace_id=%, name % is already in use',
                candidate.id, candidate.workspace_id, display_name;
            CONTINUE;
        END IF;

        BEGIN
            UPDATE agent
            SET name = display_name,
                description = CASE description
                    WHEN old_description_en THEN new_description_en
                    WHEN old_description_zh THEN new_description_zh
                    ELSE description
                END,
                updated_at = now()
            WHERE id = candidate.id
              AND workspace_id = candidate.workspace_id
              AND system_key = 'mika'
              AND name = 'Mika';
        EXCEPTION WHEN unique_violation THEN
            GET STACKED DIAGNOSTICS conflict_constraint = CONSTRAINT_NAME;
            IF conflict_constraint <> 'agent_workspace_name_unique' THEN
                RAISE;
            END IF;
            -- A concurrent owner can claim the name after the preflight check.
            RAISE NOTICE 'Skipped built-in agent rename: agent_id=%, workspace_id=%, concurrent conflict for name %',
                candidate.id, candidate.workspace_id, display_name;
        END;
    END LOOP;

    -- Also normalize unchanged descriptions on instances already using the brand.
    -- Conflicted and custom-named built-ins retain their entire saved row.
    UPDATE agent
    SET description = CASE description
            WHEN old_description_en THEN new_description_en
            WHEN old_description_zh THEN new_description_zh
        END,
        updated_at = now()
    WHERE system_key = 'mika'
      AND name = display_name
      AND description IN (old_description_en, old_description_zh);

    UPDATE chat_session AS session
    SET title = CASE session.title
            WHEN 'Getting started with Mika' THEN 'Getting started with 小阿孚'
            WHEN '和 Mika 开始' THEN '开始使用小阿孚'
        END,
        updated_at = now()
    FROM agent
    WHERE agent.id = session.agent_id
      AND agent.workspace_id = session.workspace_id
      AND agent.system_key = 'mika'
      AND agent.name = display_name
      AND session.title IN ('Getting started with Mika', '和 Mika 开始');
END;
$migration$;
