-- Budget records are retained evidence. The tables are dropped only while all
-- six are empty; the lock keeps a writer from adding a row between the check
-- and the drop.
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
DROP TABLE IF EXISTS issue_budget_observation;
DROP TABLE IF EXISTS issue_budget_operation;
DROP TABLE IF EXISTS issue_budget_entry;
DROP TABLE IF EXISTS issue_budget_attempt;
DROP TABLE IF EXISTS issue_budget_member;
DROP TABLE IF EXISTS issue_budget_root;
