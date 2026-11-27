DO $$ BEGIN
 LOCK TABLE git_material_operation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM git_material_operation WHERE receipt_history <> '{}') THEN RAISE EXCEPTION 'material observation history must be retained'; END IF;
 ALTER TABLE git_material_operation DROP COLUMN receipt_history;
END $$;
