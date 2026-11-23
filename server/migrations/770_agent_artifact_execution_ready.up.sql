DO $$
BEGIN
    IF NOT (
SELECT count(*) = 3 AS ready
FROM pg_catalog.pg_index AS idx
JOIN pg_catalog.pg_class AS index_relation ON index_relation.oid = idx.indexrelid
JOIN pg_catalog.pg_am AS method ON method.oid = index_relation.relam
WHERE idx.indrelid = to_regclass('agent_artifact_execution')
  AND index_relation.relnamespace = (SELECT relnamespace FROM pg_catalog.pg_class WHERE oid = to_regclass('agent_artifact_execution'))
  AND idx.indisunique AND idx.indisvalid AND idx.indisready AND idx.indislive
  AND NOT idx.indisexclusion AND idx.indpred IS NULL AND idx.indexprs IS NULL
  AND method.amname = 'btree'
  AND ((index_relation.relname = 'agent_artifact_execution_id_idx' AND idx.indnkeyatts = 1 AND idx.indnatts = 1 AND idx.indoption::text = '0'
        AND pg_get_indexdef(idx.indexrelid, 1, true) = 'execution_id')
    OR (index_relation.relname = 'agent_artifact_execution_approval_idx' AND idx.indnkeyatts = 1 AND idx.indnatts = 1 AND idx.indoption::text = '0'
        AND pg_get_indexdef(idx.indexrelid, 1, true) = 'approval_id')
    OR (index_relation.relname = 'agent_artifact_execution_idempotency_idx' AND idx.indnkeyatts = 4 AND idx.indnatts = 4 AND idx.indoption::text = '0 0 0 0'
        AND pg_get_indexdef(idx.indexrelid, 1, true) = 'workspace_id' AND pg_get_indexdef(idx.indexrelid, 2, true) = 'initiator_kind' AND pg_get_indexdef(idx.indexrelid, 3, true) = 'initiator_id' AND pg_get_indexdef(idx.indexrelid, 4, true) = 'idempotency_key'))
    ) THEN
        RAISE EXCEPTION 'controlled artifact indexes are absent, invalid, or have the wrong shape';
    END IF;
END;
$$;
