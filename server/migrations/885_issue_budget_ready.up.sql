-- The index migrations use IF NOT EXISTS, which accepts any index of the same
-- name. This step fails the upgrade unless each one is a valid plain btree on
-- the expected columns with the expected uniqueness and the expected
-- predicate: none, except for the two partial indexes, of the open attempts
-- and of the coverage gaps.
--
-- A predicate is compared without parentheses, whitespace and text casts, so
-- the check does not depend on how a PostgreSQL release prints it (17 prints
-- "(kind = 'coverage_gap'::text)").
DO $$
DECLARE spec record;
BEGIN
 FOR spec IN SELECT * FROM (VALUES
  ('issue_budget_root', 'issue_budget_root_id_key', true, 'id', NULL),
  ('issue_budget_root', 'issue_budget_root_issue_key', true, 'workspace_id, root_issue_id', NULL),
  ('issue_budget_member', 'issue_budget_member_issue_key', true, 'workspace_id, issue_id', NULL),
  ('issue_budget_member', 'issue_budget_member_root_idx', false, 'budget_root_id', NULL),
  ('issue_budget_attempt', 'issue_budget_attempt_id_key', true, 'id', NULL),
  ('issue_budget_attempt', 'issue_budget_attempt_claim_key', true, 'task_id, claim_generation', NULL),
  ('issue_budget_attempt', 'issue_budget_attempt_root_idx', false, 'budget_root_id', NULL),
  ('issue_budget_attempt', 'issue_budget_attempt_open_idx', false, 'budget_root_id', 'state=''open'''),
  ('issue_budget_entry', 'issue_budget_entry_seq_key', true, 'budget_root_id, seq', NULL),
  ('issue_budget_entry', 'issue_budget_entry_identity_key', true, 'budget_root_id, entry_id', NULL),
  ('issue_budget_entry', 'issue_budget_entry_attempt_idx', false, 'attempt_id', NULL),
  ('issue_budget_operation', 'issue_budget_operation_id_key', true, 'id', NULL),
  ('issue_budget_operation', 'issue_budget_operation_scope_key', true, 'workspace_id, scope_kind, scope_id, operation_key', NULL),
  ('issue_budget_observation', 'issue_budget_observation_root_idx', false, 'budget_root_id', NULL),
  ('issue_budget_observation', 'issue_budget_observation_gap_idx', false, 'budget_root_id, reason', 'kind=''coverage_gap''')
 ) AS v(table_name, index_name, is_unique, key_columns, predicate) LOOP
  IF NOT EXISTS (
   SELECT 1 FROM pg_catalog.pg_index i
   JOIN pg_catalog.pg_class c ON c.oid = i.indexrelid
   JOIN pg_catalog.pg_am a ON a.oid = c.relam
   WHERE i.indrelid = to_regclass(spec.table_name) AND c.relname = spec.index_name AND a.amname = 'btree'
   AND i.indisunique = spec.is_unique AND i.indisvalid AND i.indisready AND i.indislive
   AND i.indexprs IS NULL AND i.indnkeyatts = i.indnatts
   AND regexp_replace(pg_catalog.pg_get_expr(i.indpred, i.indrelid), '[()[:space:]]|::text', '', 'g')
       IS NOT DISTINCT FROM spec.predicate
   AND (SELECT string_agg(pg_catalog.pg_get_indexdef(i.indexrelid, k, true), ', ' ORDER BY k)
        FROM generate_series(1, i.indnkeyatts::integer) AS k) = spec.key_columns
  ) THEN
   RAISE EXCEPTION 'issue budget index % is missing, invalid or has the wrong shape', spec.index_name;
  END IF;
 END LOOP;
END; $$;
