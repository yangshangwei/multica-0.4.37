ALTER TABLE "user" ALTER COLUMN email DROP NOT NULL;
CREATE TABLE user_password_credential (
 user_id UUID NOT NULL,
 username TEXT NOT NULL,
 password_hash TEXT NOT NULL,
 session_version BIGINT NOT NULL DEFAULT 1 CHECK (session_version > 0),
 must_change_password BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE personal_access_token ADD COLUMN auth_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE task_token ADD COLUMN auth_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE daemon_token ADD COLUMN user_id UUID, ADD COLUMN auth_version BIGINT NOT NULL DEFAULT 0;
