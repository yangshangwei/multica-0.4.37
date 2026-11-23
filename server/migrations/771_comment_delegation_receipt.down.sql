DO $$ BEGIN
 IF to_regclass('comment_delegation_receipt') IS NOT NULL THEN
  LOCK TABLE comment_delegation_receipt IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM comment_delegation_receipt) THEN
   RAISE EXCEPTION 'cannot roll back retained delegation evidence';
  END IF;
 END IF;
END; $$;
DROP TABLE IF EXISTS comment_delegation_receipt;
