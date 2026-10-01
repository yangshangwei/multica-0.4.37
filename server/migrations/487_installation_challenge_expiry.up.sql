CREATE INDEX CONCURRENTLY installation_challenge_expiry_idx ON installation_challenge (expires_at, id);
