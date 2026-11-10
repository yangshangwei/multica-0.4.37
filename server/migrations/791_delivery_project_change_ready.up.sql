DO $$ BEGIN
 IF NOT (SELECT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('delivery_project_change') AND c.relname='delivery_project_change_operation_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3 AND i.indoption::text='0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='operation_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='issue_id'
)) THEN RAISE EXCEPTION 'delivery project change schema unavailable'; END IF;
END; $$;
