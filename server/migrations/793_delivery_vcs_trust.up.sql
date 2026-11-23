ALTER TABLE vcs_connection
 ADD COLUMN trust_revision bigint NOT NULL DEFAULT 1
 CONSTRAINT vcs_connection_trust_revision_positive CHECK (trust_revision > 0);
