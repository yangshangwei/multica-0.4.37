DO $$ BEGIN
 LOCK TABLE git_material_operation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM git_material_operation) THEN RAISE EXCEPTION 'material operation history must be retained'; END IF;
 DROP TABLE git_material_operation;
END $$;
