DO $$ BEGIN
 LOCK TABLE collaboration_test_subject IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_test_subject) THEN
  RAISE EXCEPTION 'test subject history is retained' USING ERRCODE='23514';
 END IF;
END $$;
DROP TABLE collaboration_test_subject;
