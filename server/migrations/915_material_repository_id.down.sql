DO $$ BEGIN
 LOCK TABLE project_material_repository IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS(SELECT 1 FROM project_material_repository) OR EXISTS(SELECT 1 FROM material_repository_revision) OR EXISTS(SELECT 1 FROM material_repository_decision) OR EXISTS(SELECT 1 FROM git_material_job WHERE target_kind='repository_initialization') OR EXISTS(SELECT 1 FROM git_material_operation WHERE target_kind='repository_initialization') THEN RAISE EXCEPTION 'repository history must be retained'; END IF;
 DROP INDEX IF EXISTS material_repository_id_idx;
END $$;
