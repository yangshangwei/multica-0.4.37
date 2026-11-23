ALTER TABLE github_installation
 ADD COLUMN trust_revision bigint NOT NULL DEFAULT 1
 CONSTRAINT github_installation_trust_revision_positive CHECK (trust_revision > 0);
