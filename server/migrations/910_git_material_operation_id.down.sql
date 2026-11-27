DO $$ BEGIN
 LOCK TABLE git_material_operation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM git_material_operation) THEN RAISE EXCEPTION 'material operation identity must be retained'; END IF;
 DROP INDEX IF EXISTS git_material_operation_id_idx;
END $$;
