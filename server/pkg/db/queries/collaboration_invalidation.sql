-- The caller holds the same user row lock as collaboration participation and
-- owns the credential/account transaction. Never scan or lock source issues
-- here: the durable intent is reconciled separately after commit.
-- Existing operation/credential identities make retries stable; missing fences
-- mean the account has never acquired collaboration authority. Bigint overflow
-- must fail the entire security mutation rather than silently skip invalidation.
-- name: InvalidateCollaborationAccount :execrows
WITH advanced AS (
    UPDATE collaboration_account_fence AS fence
    SET epoch = fence.epoch + 1, updated_at = now()
    WHERE fence.user_id = sqlc.arg(user_id)::uuid
      AND NOT EXISTS (
          SELECT 1 FROM collaboration_account_invalidation AS intent
          WHERE intent.user_id = fence.user_id
            AND intent.revocation_key = sqlc.arg(revocation_key)::text
      )
    RETURNING fence.user_id, fence.epoch
)
INSERT INTO collaboration_account_invalidation
    (user_id, account_epoch, revocation_key, credential_version, reason)
SELECT advanced.user_id, advanced.epoch, sqlc.arg(revocation_key)::text,
       credential.session_version, 'credentials_revoked'
FROM advanced
LEFT JOIN user_password_credential AS credential ON credential.user_id = advanced.user_id;
