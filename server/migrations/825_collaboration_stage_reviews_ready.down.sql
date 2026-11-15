DO $$ BEGIN
 LOCK TABLE collaboration_stage_candidate, collaboration_stage_decision, collaboration_stage_revocation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_stage_candidate WHERE kind='code')
 OR EXISTS (SELECT 1 FROM collaboration_stage_decision WHERE step IN ('code_review','qa','release','rollback') OR signer_credential IS NOT NULL)
 OR EXISTS (SELECT 1 FROM collaboration_stage_revocation) THEN
  RAISE EXCEPTION 'stage review history is retained' USING ERRCODE='23514';
 END IF;
END $$;
