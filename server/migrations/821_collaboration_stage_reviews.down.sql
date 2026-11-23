-- Never erase a standing decision or its revocation to evade stage admission.
DO $$ BEGIN
 LOCK TABLE collaboration_stage_candidate, collaboration_stage_decision, collaboration_stage_revocation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_stage_candidate WHERE kind='code')
 OR EXISTS (SELECT 1 FROM collaboration_stage_decision WHERE step IN ('code_review','qa','release','rollback') OR signer_credential IS NOT NULL)
 OR EXISTS (SELECT 1 FROM collaboration_stage_revocation) THEN
  RAISE EXCEPTION 'stage review history is retained' USING ERRCODE='23514';
 END IF;
END $$;
DROP TABLE collaboration_stage_revocation;
ALTER TABLE collaboration_stage_decision
 DROP CONSTRAINT collaboration_stage_decision_shape,
 DROP CONSTRAINT collaboration_stage_decision_step_check,
 DROP CONSTRAINT collaboration_stage_decision_decision_check,
 DROP CONSTRAINT collaboration_stage_decision_role_check,
 DROP COLUMN signer_credential, DROP COLUMN expires_at,
 ADD CONSTRAINT collaboration_stage_decision_step_check CHECK (step IN ('proposal','scope')),
 ADD CONSTRAINT collaboration_stage_decision_decision_check CHECK (decision IN ('accepted','rejected','confirmed')),
 ADD CONSTRAINT collaboration_stage_decision_role_check CHECK (role='product_approver'),
 ADD CONSTRAINT collaboration_stage_decision_shape CHECK (
  (step='proposal' AND decision IN ('accepted','rejected') AND proposal_id IS NOT NULL
   AND candidate_id IS NOT NULL AND candidate_version IS NOT NULL AND candidate_version>0 AND content_digest IS NOT NULL)
  OR (step='scope' AND decision='confirmed' AND proposal_id IS NULL
   AND candidate_id IS NULL AND candidate_version IS NULL AND content_digest IS NULL));
ALTER TABLE collaboration_stage_candidate DROP CONSTRAINT collaboration_stage_candidate_family_check,
 DROP CONSTRAINT collaboration_stage_candidate_kind_check,
 ALTER COLUMN proposal_id SET NOT NULL,
 ADD CONSTRAINT collaboration_stage_candidate_kind_check CHECK (kind='proposal');
ALTER TABLE github_pull_request DROP COLUMN snapshot_complete;
