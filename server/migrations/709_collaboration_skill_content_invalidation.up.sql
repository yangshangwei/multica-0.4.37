BEGIN;

-- Fence existing skill writers while installing the new authority boundary.
LOCK TABLE skill, skill_file, agent_skill IN SHARE ROW EXCLUSIVE MODE;

-- Earlier epochs did not include shared content. Conservatively retire that
-- consent once, retaining every original target/group/run/proof identity.
DO $$
DECLARE parent uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='skill'::regclass AND tgname='collaboration_skill_content_config') THEN
        FOR parent IN
            SELECT DISTINCT binding.agent_id FROM agent_skill binding
            JOIN skill s ON s.id=binding.skill_id
            WHERE binding.enabled AND (
                EXISTS (SELECT 1 FROM collaboration_grant g WHERE g.agent_id=binding.agent_id)
                OR EXISTS (SELECT 1 FROM collaboration_target t, LATERAL jsonb_array_elements(t.roster) resource
                    WHERE resource->>'agent_id'=binding.agent_id::text)
            ) ORDER BY binding.agent_id
        LOOP
            PERFORM id FROM agent WHERE id=parent FOR UPDATE NOWAIT;
            UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE id=parent;
        END LOOP;
    END IF;
END;
$$;

-- Both skill content and supporting files invalidate every enabled binding.
-- A skill writer already owns its skill fence; never wait for an agent while
-- owning that fence, because admission and binding writers lock agents first.
CREATE OR REPLACE FUNCTION collaboration_advance_skill_agents(skills uuid[]) RETURNS void LANGUAGE plpgsql AS $$
DECLARE parent uuid;
BEGIN
    FOR parent IN SELECT DISTINCT agent_id FROM agent_skill
        WHERE skill_id=ANY(skills) AND enabled ORDER BY agent_id
    LOOP
        PERFORM id FROM agent WHERE id=parent FOR UPDATE NOWAIT;
        UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE id=parent;
    END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION collaboration_record_skill_content_config() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='UPDATE' THEN
        -- Name and description enter the runtime bundle. Only presentation is
        -- display-only; origin/version and unknown config keys remain fenced.
        IF ROW(NEW.workspace_id, NEW.name, NEW.description, NEW.content, NEW.plugin_installation_id,
            CASE WHEN jsonb_typeof(NEW.config)='object' THEN NEW.config-'presentation' ELSE NEW.config END)
            IS NOT DISTINCT FROM ROW(OLD.workspace_id, OLD.name, OLD.description, OLD.content, OLD.plugin_installation_id,
            CASE WHEN jsonb_typeof(OLD.config)='object' THEN OLD.config-'presentation' ELSE OLD.config END)
        THEN RETURN NEW; END IF;
    END IF;
    PERFORM collaboration_advance_skill_agents(ARRAY[OLD.id]);
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_skill_content_config ON skill;
CREATE TRIGGER collaboration_skill_content_config AFTER UPDATE ON skill
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_skill_content_config();
DROP TRIGGER IF EXISTS collaboration_skill_delete_config ON skill;
CREATE TRIGGER collaboration_skill_delete_config BEFORE DELETE ON skill
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_skill_content_config();

-- Bind/enable must see a complete committed bundle. SHARE conflicts with the
-- skill UPDATE lock; KEY SHARE would not. Files take UPDATE to serialize all
-- file and content writes of that skill, even a file-only API request.
CREATE OR REPLACE FUNCTION collaboration_fence_skill_child() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE skills uuid[] := '{}'; parent uuid;
BEGIN
    IF TG_OP<>'INSERT' THEN skills:=array_append(skills, OLD.skill_id); END IF;
    IF TG_OP<>'DELETE' THEN skills:=array_append(skills, NEW.skill_id); END IF;
    FOR parent IN SELECT DISTINCT id FROM unnest(skills) changed(id) ORDER BY id LOOP
        IF TG_TABLE_NAME='agent_skill' THEN
            PERFORM id FROM skill WHERE id=parent FOR SHARE NOWAIT;
        ELSE
            PERFORM id FROM skill WHERE id=parent FOR UPDATE NOWAIT;
        END IF;
    END LOOP;
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_skill_file_fence ON skill_file;
CREATE TRIGGER collaboration_skill_file_fence BEFORE INSERT OR UPDATE OR DELETE ON skill_file
    FOR EACH ROW EXECUTE FUNCTION collaboration_fence_skill_child();
DROP TRIGGER IF EXISTS collaboration_skill_binding_fence ON agent_skill;
CREATE TRIGGER collaboration_skill_binding_fence BEFORE INSERT OR UPDATE OR DELETE ON agent_skill
    FOR EACH ROW EXECUTE FUNCTION collaboration_fence_skill_child();

CREATE OR REPLACE FUNCTION collaboration_record_skill_file_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE skills uuid[] := '{}';
BEGIN
    IF TG_OP='UPDATE' AND ROW(NEW.skill_id, NEW.path, NEW.content) IS NOT DISTINCT FROM ROW(OLD.skill_id, OLD.path, OLD.content) THEN
        RETURN NULL;
    END IF;
    IF TG_OP<>'INSERT' THEN skills:=array_append(skills, OLD.skill_id); END IF;
    IF TG_OP<>'DELETE' THEN skills:=array_append(skills, NEW.skill_id); END IF;
    -- A parent DELETE already captured its bindings before historical cascade
    -- cleanup; do not duplicate invalidation as its child rows disappear.
    PERFORM collaboration_advance_skill_agents(ARRAY(SELECT id FROM skill WHERE id=ANY(skills)));
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_skill_file_config ON skill_file;
CREATE TRIGGER collaboration_skill_file_config AFTER INSERT OR UPDATE OR DELETE ON skill_file
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_skill_file_config();

CREATE OR REPLACE FUNCTION collaboration_record_skill_binding_config() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parents uuid[] := '{}'; parent uuid;
BEGIN
    IF TG_OP='UPDATE' AND ROW(NEW.agent_id, NEW.skill_id, NEW.enabled) IS NOT DISTINCT FROM ROW(OLD.agent_id, OLD.skill_id, OLD.enabled) THEN
        RETURN NULL;
    END IF;
    IF TG_OP<>'INSERT' AND OLD.enabled AND EXISTS(SELECT 1 FROM skill WHERE id=OLD.skill_id) THEN
        parents:=array_append(parents, OLD.agent_id);
    END IF;
    IF TG_OP<>'DELETE' AND NEW.enabled THEN parents:=array_append(parents, NEW.agent_id); END IF;
    FOR parent IN SELECT DISTINCT id FROM unnest(parents) changed(id) ORDER BY id LOOP
        PERFORM id FROM agent WHERE id=parent FOR UPDATE NOWAIT;
        UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE id=parent;
    END LOOP;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_agent_skill_config ON agent_skill;
CREATE TRIGGER collaboration_agent_skill_config AFTER INSERT OR UPDATE OR DELETE ON agent_skill
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_skill_binding_config();

-- Plugin upgrades identify their release on the installation, not skill.config.
-- A new release needs fresh consent even when it ships identical SKILL.md bytes.
CREATE OR REPLACE FUNCTION collaboration_record_plugin_skill_version_config() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.package_version_id IS DISTINCT FROM OLD.package_version_id THEN
        PERFORM id FROM skill WHERE plugin_installation_id=NEW.id ORDER BY id FOR UPDATE NOWAIT;
        PERFORM collaboration_advance_skill_agents(ARRAY(
            SELECT id FROM skill WHERE plugin_installation_id=NEW.id ORDER BY id));
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS collaboration_plugin_skill_version_config ON plugin_installation;
CREATE TRIGGER collaboration_plugin_skill_version_config AFTER UPDATE ON plugin_installation
    FOR EACH ROW EXECUTE FUNCTION collaboration_record_plugin_skill_version_config();

COMMIT;
