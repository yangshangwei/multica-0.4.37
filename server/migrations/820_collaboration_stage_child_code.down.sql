-- First step of a rollback: refuse while any stage history is retained, so a
-- refused rollback changes nothing. Only an empty schema loses these columns.
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
 IF to_regclass('collaboration_stage_child') IS NOT NULL THEN
  ALTER TABLE collaboration_stage_child DROP CONSTRAINT IF EXISTS collaboration_stage_child_code;
  ALTER TABLE collaboration_stage_child DROP COLUMN IF EXISTS code_provider,
   DROP COLUMN IF EXISTS code_pull_request_id, DROP COLUMN IF EXISTS code_head_sha;
 END IF;
END; $$;
