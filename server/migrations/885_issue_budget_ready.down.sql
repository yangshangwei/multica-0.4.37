-- First rollback step. Budget records are retained evidence, so the rollback
-- stops here, before any index is dropped, while any of the six tables holds a
-- row. 869 repeats the check under the same locks before it drops the tables.
DO $$ BEGIN
 IF to_regclass('issue_budget_root') IS NOT NULL THEN
  LOCK TABLE issue_budget_root IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_budget_root) THEN
   RAISE EXCEPTION 'cannot roll back retained budget records: issue_budget_root has rows';
  END IF;
 END IF;
 IF to_regclass('issue_budget_member') IS NOT NULL THEN
  LOCK TABLE issue_budget_member IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_budget_member) THEN
   RAISE EXCEPTION 'cannot roll back retained budget records: issue_budget_member has rows';
  END IF;
 END IF;
 IF to_regclass('issue_budget_attempt') IS NOT NULL THEN
  LOCK TABLE issue_budget_attempt IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_budget_attempt) THEN
   RAISE EXCEPTION 'cannot roll back retained budget records: issue_budget_attempt has rows';
  END IF;
 END IF;
 IF to_regclass('issue_budget_entry') IS NOT NULL THEN
  LOCK TABLE issue_budget_entry IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_budget_entry) THEN
   RAISE EXCEPTION 'cannot roll back retained budget records: issue_budget_entry has rows';
  END IF;
 END IF;
 IF to_regclass('issue_budget_operation') IS NOT NULL THEN
  LOCK TABLE issue_budget_operation IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_budget_operation) THEN
   RAISE EXCEPTION 'cannot roll back retained budget records: issue_budget_operation has rows';
  END IF;
 END IF;
 IF to_regclass('issue_budget_observation') IS NOT NULL THEN
  LOCK TABLE issue_budget_observation IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_budget_observation) THEN
   RAISE EXCEPTION 'cannot roll back retained budget records: issue_budget_observation has rows';
  END IF;
 END IF;
END; $$;
