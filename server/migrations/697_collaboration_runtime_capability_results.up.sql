-- Exact replay must recover the same capabilities. The historical runtime
-- identity remains fixed even when Stop advances the live claim generation.
ALTER TABLE collaboration_termination ADD COLUMN runtime_identity jsonb CHECK (runtime_identity IS NULL OR jsonb_typeof(runtime_identity)='object'),
    ADD COLUMN token_ciphertext bytea;
ALTER TABLE collaboration_business_capability ADD COLUMN token_ciphertext bytea;
ALTER TABLE collaboration_run ADD COLUMN deployment_id uuid, ADD COLUMN resource_owner_user_id uuid,
    ADD COLUMN containment text;
