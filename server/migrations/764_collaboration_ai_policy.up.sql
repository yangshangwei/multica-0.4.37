-- Task policy is independent of deployment admission, target holds and proof.
-- Existing collaborations retain their assisted behavior on upgrade.
ALTER TABLE issue_collaboration
  ADD COLUMN ai_policy TEXT NOT NULL DEFAULT 'assisted'
  CHECK (ai_policy IN ('manual', 'assisted'));
