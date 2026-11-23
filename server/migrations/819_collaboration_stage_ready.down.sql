-- First step of a rollback: refuse before any index is dropped while stage
-- history is retained. Disable the project policy instead of deleting audit rows.
DO $$ BEGIN
 IF to_regclass('collaboration_stage_child') IS NOT NULL THEN
  LOCK TABLE collaboration_stage_child IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('collaboration_stage_decision') IS NOT NULL THEN
  LOCK TABLE collaboration_stage_decision IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('collaboration_stage_candidate') IS NOT NULL THEN
  LOCK TABLE collaboration_stage_candidate IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('collaboration_stage_role') IS NOT NULL THEN
  LOCK TABLE collaboration_stage_role IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('collaboration_stage_task') IS NOT NULL THEN
  LOCK TABLE collaboration_stage_task IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('project_stage_policy') IS NOT NULL THEN
  LOCK TABLE project_stage_policy IN ACCESS EXCLUSIVE MODE NOWAIT;
 END IF;
 IF to_regclass('collaboration_stage_child') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM collaboration_stage_child) THEN
   RAISE EXCEPTION 'cannot roll back retained collaboration stage records (collaboration_stage_child)';
  END IF;
 END IF;
 IF to_regclass('collaboration_stage_decision') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM collaboration_stage_decision) THEN
   RAISE EXCEPTION 'cannot roll back retained collaboration stage records (collaboration_stage_decision)';
  END IF;
 END IF;
 IF to_regclass('collaboration_stage_candidate') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM collaboration_stage_candidate) THEN
   RAISE EXCEPTION 'cannot roll back retained collaboration stage records (collaboration_stage_candidate)';
  END IF;
 END IF;
 IF to_regclass('collaboration_stage_role') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM collaboration_stage_role) THEN
   RAISE EXCEPTION 'cannot roll back retained collaboration stage records (collaboration_stage_role)';
  END IF;
 END IF;
 IF to_regclass('collaboration_stage_task') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM collaboration_stage_task) THEN
   RAISE EXCEPTION 'cannot roll back retained collaboration stage records (collaboration_stage_task)';
  END IF;
 END IF;
 IF to_regclass('project_stage_policy') IS NOT NULL THEN
  IF EXISTS (SELECT 1 FROM project_stage_policy) THEN
   RAISE EXCEPTION 'cannot roll back retained collaboration stage records (project_stage_policy)';
  END IF;
 END IF;
END; $$;
