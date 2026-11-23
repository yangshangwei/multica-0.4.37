DO $$ BEGIN
 IF to_regclass('delivery_completion_authorization') IS NOT NULL THEN
  LOCK TABLE delivery_completion_authorization IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM delivery_completion_authorization) THEN RAISE EXCEPTION 'cannot roll back retained delivery completion authorizations'; END IF;
 END IF;
 IF to_regclass('delivery_completion_terminal') IS NOT NULL THEN
  LOCK TABLE delivery_completion_terminal IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM delivery_completion_terminal) THEN RAISE EXCEPTION 'cannot roll back retained delivery completion terminal facts'; END IF;
 END IF;
 IF to_regclass('project_delivery_policy') IS NOT NULL THEN
  LOCK TABLE project_delivery_policy IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM project_delivery_policy WHERE mode='enforce') THEN RAISE EXCEPTION 'cannot roll back retained enforce policy history'; END IF;
 END IF;
 DROP TABLE IF EXISTS delivery_completion_terminal;
 DROP TABLE IF EXISTS delivery_completion_authorization;
END; $$;
