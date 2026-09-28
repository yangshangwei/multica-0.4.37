-- Older binaries do not serve the seal endpoint. Restore only that default;
-- preserve custom and cleared avatars, including changes made after migration.
UPDATE agent
SET avatar_url = 'emoji:🦄', updated_at = now()
WHERE system_key = 'mika'
  AND avatar_url = '/api/avatars/builtin/afu-seal-v1.png';
