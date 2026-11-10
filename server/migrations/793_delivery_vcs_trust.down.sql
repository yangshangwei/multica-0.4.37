DO $$ BEGIN
 LOCK TABLE vcs_connection IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF to_regclass('delivery_completion_authorization') IS NOT NULL THEN
  LOCK TABLE delivery_completion_authorization IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM delivery_completion_authorization WHERE consumer_kind IN ('github','vcs')) THEN
   RAISE EXCEPTION 'cannot roll back retained VCS completion authorization trust';
  END IF;
 END IF;
 ALTER TABLE vcs_connection DROP COLUMN IF EXISTS trust_revision;
END; $$;
