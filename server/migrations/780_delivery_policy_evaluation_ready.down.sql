DO $$ BEGIN
 IF to_regclass('delivery_evidence_evaluation') IS NOT NULL THEN
  LOCK TABLE delivery_evidence_evaluation IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('project_delivery_policy') IS NOT NULL THEN
  LOCK TABLE project_delivery_policy IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('delivery_evidence_evaluation') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM delivery_evidence_evaluation) THEN
   RAISE EXCEPTION 'cannot roll back retained delivery evaluations';
  END IF;
 END IF;
 IF to_regclass('project_delivery_policy') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM project_delivery_policy) THEN
   RAISE EXCEPTION 'cannot roll back retained delivery policies';
  END IF;
 END IF;
END; $$;
