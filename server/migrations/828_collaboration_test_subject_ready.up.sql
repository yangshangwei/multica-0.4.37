DO $$ BEGIN
 IF NOT EXISTS (
  SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid
  JOIN pg_catalog.pg_am a ON a.oid=c.relam
  WHERE i.indrelid='collaboration_test_subject'::regclass AND c.relname='collaboration_test_subject_command_idx'
   AND a.amname='btree' AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
   AND NOT i.indisexclusion AND i.indexprs IS NULL AND i.indpred IS NULL AND i.indnkeyatts=1 AND i.indnatts=1
   AND pg_get_indexdef(i.indexrelid,1,true)='command_id'
 ) THEN
  RAISE EXCEPTION 'test subject index is unavailable' USING ERRCODE='23514';
 END IF;
END $$;
