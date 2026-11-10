DO $$ BEGIN
 IF NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('comment_delegation_receipt')
 AND c.relname='comment_delegation_receipt_key_idx' AND a.amname='btree'
 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indpred IS NULL AND i.indexprs IS NULL
 AND i.indnkeyatts=5 AND i.indnatts=5 AND i.indoption::text='0 0 0 0 0'
 AND pg_get_indexdef(i.indexrelid,1,true)='workspace_id'
 AND pg_get_indexdef(i.indexrelid,2,true)='actor_type'
 AND pg_get_indexdef(i.indexrelid,3,true)='actor_id'
 AND pg_get_indexdef(i.indexrelid,4,true)='originator_id'
 AND pg_get_indexdef(i.indexrelid,5,true)='request_key'
 ) THEN RAISE EXCEPTION 'delegation receipt unique index is absent, invalid or wrong shape'; END IF;
END; $$;
