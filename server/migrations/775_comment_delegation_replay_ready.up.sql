DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid
 WHERE i.indrelid=to_regclass('comment_delegation_receipt') AND c.relname='comment_delegation_receipt_comment_idx'
 AND i.indisvalid AND i.indisready AND i.indislive AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=3 AND i.indnatts=3
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id' AND pg_get_indexdef(i.indexrelid,2,true)='issue_id'
 AND pg_get_indexdef(i.indexrelid,3,true)='comment_id')
 THEN RAISE EXCEPTION 'delegation replay lookup index missing or invalid'; END IF;
END; $$;
