DO $$ BEGIN
 LOCK TABLE github_installation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF to_regclass('delivery_completion_authorization') IS NOT NULL THEN
  LOCK TABLE delivery_completion_authorization IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM delivery_completion_authorization WHERE consumer_kind IN ('github','vcs')) THEN
   RAISE EXCEPTION 'cannot roll back retained GitHub completion authorization trust';
  END IF;
 END IF;
 ALTER TABLE github_installation DROP COLUMN IF EXISTS trust_revision;
END; $$;
