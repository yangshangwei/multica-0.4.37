DO $$ BEGIN
 IF NOT (EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('project_delivery_policy') AND c.relname='project_delivery_policy_revision_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3 AND i.indoption::text='0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='project_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='revision'
) AND
EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('delivery_evidence_evaluation') AND c.relname='delivery_evaluation_key_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3 AND i.indoption::text='0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='evaluator_user_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='request_key'
) AND
EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('delivery_evidence_evaluation') AND c.relname='delivery_evaluation_id_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=1 AND i.indnatts=1 AND i.indoption::text='0'
 AND pg_get_indexdef(i.indexrelid,1,true)='evaluation_id'
)) THEN RAISE EXCEPTION 'delivery policy and evaluation indexes are absent, invalid or wrong shape'; END IF;
END; $$;
