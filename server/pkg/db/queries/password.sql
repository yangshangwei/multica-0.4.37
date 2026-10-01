-- name: GetPasswordCredential :one
SELECT * FROM user_password_credential WHERE user_id = $1;

-- name: GetPasswordCredentialByUsername :one
SELECT * FROM user_password_credential WHERE username = $1;

-- name: CreatePasswordCredential :one
INSERT INTO user_password_credential (user_id, username, password_hash, must_change_password)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ChangePasswordCredential :one
UPDATE user_password_credential SET password_hash = $2, session_version = session_version + 1,
 must_change_password = $3, updated_at = now()
WHERE user_id = $1 AND session_version = $4 AND session_version < 9223372036854775807 RETURNING *;

-- name: LockPasswordUser :one
SELECT id FROM "user" WHERE id = $1 FOR UPDATE;
