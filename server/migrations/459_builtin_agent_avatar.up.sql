-- Replace only the built-in agent's untouched placeholder avatar.
UPDATE agent
SET avatar_url = '/api/avatars/builtin/afu-seal-v1.png', updated_at = now()
WHERE system_key = 'mika'
  AND avatar_url = 'emoji:🦄';
