-- Downgrade must not silently make a retained manual task AI-enabled.
DO $$
BEGIN
  LOCK TABLE issue_collaboration IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF EXISTS (SELECT 1 FROM issue_collaboration WHERE ai_policy = 'manual') THEN
    RAISE EXCEPTION 'cannot remove collaboration manual policy while manual tasks remain';
  END IF;
  ALTER TABLE issue_collaboration DROP COLUMN ai_policy;
END $$;
