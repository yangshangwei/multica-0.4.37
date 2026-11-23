-- Standing decisions extend the existing immutable stage ledger. No bearer
-- credentials are retained: signer_credential holds verified identity metadata.
ALTER TABLE collaboration_stage_candidate
 DROP CONSTRAINT collaboration_stage_candidate_kind_check,
 ALTER COLUMN proposal_id DROP NOT NULL,
 ADD CONSTRAINT collaboration_stage_candidate_kind_check CHECK (kind IN ('proposal','code')),
 ADD CONSTRAINT collaboration_stage_candidate_family_check CHECK ((kind='proposal') = (proposal_id IS NOT NULL));

ALTER TABLE collaboration_stage_decision
 DROP CONSTRAINT collaboration_stage_decision_step_check,
 DROP CONSTRAINT collaboration_stage_decision_decision_check,
 DROP CONSTRAINT collaboration_stage_decision_role_check,
 DROP CONSTRAINT collaboration_stage_decision_shape,
 ADD COLUMN signer_credential jsonb,
 ADD COLUMN expires_at timestamptz,
 ADD CONSTRAINT collaboration_stage_decision_step_check CHECK (step IN ('proposal','scope','code_review','qa','release','rollback')),
 ADD CONSTRAINT collaboration_stage_decision_decision_check CHECK (decision IN ('accepted','rejected','confirmed','approved','returned')),
 ADD CONSTRAINT collaboration_stage_decision_role_check CHECK (role IN ('product_approver','code_reviewer','qa_approver','release_approver')),
 ADD CONSTRAINT collaboration_stage_decision_shape CHECK (
  (step='proposal' AND role='product_approver' AND decision IN ('accepted','rejected') AND proposal_id IS NOT NULL
   AND candidate_id IS NOT NULL AND candidate_version IS NOT NULL AND candidate_version>0 AND content_digest IS NOT NULL AND expires_at IS NULL)
  OR (step='scope' AND role='product_approver' AND decision='confirmed' AND proposal_id IS NULL
   AND candidate_id IS NULL AND candidate_version IS NULL AND content_digest IS NULL AND expires_at IS NULL)
  OR (step IN ('code_review','qa','release','rollback') AND proposal_id IS NULL
   AND candidate_id IS NOT NULL AND candidate_version IS NOT NULL AND candidate_version>0 AND content_digest IS NOT NULL
   AND signer_credential IS NOT NULL AND jsonb_typeof(signer_credential)='object'
   AND role=CASE step WHEN 'code_review' THEN 'code_reviewer' WHEN 'qa' THEN 'qa_approver' ELSE 'release_approver' END
   AND ((decision='approved' AND expires_at IS NOT NULL AND expires_at>created_at)
     OR (decision='returned' AND expires_at IS NULL))));

CREATE TABLE collaboration_stage_revocation (
 revocation_id uuid NOT NULL, decision_id uuid NOT NULL,
 issue_id uuid NOT NULL, workspace_id uuid NOT NULL,
 actor_user_id uuid NOT NULL, operation_id uuid NOT NULL,
 reason text NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 1 AND 2000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TRIGGER collaboration_stage_revocation_immutable BEFORE UPDATE ON collaboration_stage_revocation
 FOR EACH ROW EXECUTE FUNCTION collaboration_stage_immutable();

-- Older snapshots did not prove that every union node was normalized. They
-- must be refreshed through the fixed producer before satisfying strict CI.
ALTER TABLE github_pull_request ADD COLUMN snapshot_complete boolean NOT NULL DEFAULT false;
