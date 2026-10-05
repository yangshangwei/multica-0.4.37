-- Retain source provenance on rollback. Removing it could misidentify saved
-- deployment instances as builtin templates when a newer server is restored.
SELECT 1;
