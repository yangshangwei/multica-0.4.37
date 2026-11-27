DO $$ BEGIN
 LOCK TABLE project_material_repository,material_repository_revision,material_repository_decision IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS(SELECT 1 FROM project_material_repository) OR EXISTS(SELECT 1 FROM material_repository_revision) OR EXISTS(SELECT 1 FROM material_repository_decision) OR EXISTS(SELECT 1 FROM git_material_job WHERE target_kind='repository_initialization') OR EXISTS(SELECT 1 FROM git_material_operation WHERE target_kind='repository_initialization') THEN RAISE EXCEPTION 'repository history must be retained'; END IF;
 ALTER TABLE git_material_job ALTER COLUMN material_id SET NOT NULL, ALTER COLUMN revision_id SET NOT NULL;
 ALTER TABLE git_material_operation ALTER COLUMN material_id SET NOT NULL, ALTER COLUMN revision_id SET NOT NULL;
 DROP TABLE material_repository_decision,material_repository_revision,project_material_repository;
 ALTER TABLE git_material_job DROP COLUMN target_kind, DROP COLUMN repository_id, DROP COLUMN repository_revision_id;
 ALTER TABLE git_material_operation DROP COLUMN target_kind, DROP COLUMN repository_id, DROP COLUMN repository_revision_id;
END $$;
