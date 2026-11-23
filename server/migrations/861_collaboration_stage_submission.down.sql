DO $$ BEGIN
 LOCK TABLE collaboration_stage_submission IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_stage_submission) THEN
 RAISE EXCEPTION 'stage submission history is retained' USING ERRCODE='23514';
 END IF;
END $$;
DROP TABLE collaboration_stage_submission;
