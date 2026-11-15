-- Stop stage admission for the entire multi-file downgrade. Retained facts
-- refuse before any supporting index is removed.
DO $$ BEGIN
 LOCK TABLE collaboration_stage_submission IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_stage_submission) THEN
 RAISE EXCEPTION 'stage submission history is retained' USING ERRCODE='23514';
 END IF;
END $$;
