-- Refuse before dropping the concurrent index. Admission must be stopped
-- throughout a multi-file downgrade, as for the other stage ledgers.
DO $$ BEGIN
 LOCK TABLE collaboration_test_subject IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_test_subject) THEN
  RAISE EXCEPTION 'test subject history is retained' USING ERRCODE='23514';
 END IF;
END $$;
