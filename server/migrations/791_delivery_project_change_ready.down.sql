DO $$ BEGIN
 IF to_regclass('delivery_project_change') IS NOT NULL THEN
  LOCK TABLE delivery_project_change IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS(SELECT 1 FROM delivery_project_change) THEN RAISE EXCEPTION 'cannot roll back retained delivery project changes'; END IF;
 END IF;
END; $$;
