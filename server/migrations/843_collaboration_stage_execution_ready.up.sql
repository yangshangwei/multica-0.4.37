DO $$ BEGIN
 IF NOT (SELECT NOT EXISTS (
 SELECT 1 FROM (VALUES
  ('collaboration_stage_execution_intent','collaboration_stage_execution_intent_id_idx',true,'intent_id',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_approval_idx',true,'approval_decision_id',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_request_idx',true,'workspace_id,consumer_subject,request_id',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_target_idx',false,'physical_target_key,admitted_at',''),
  ('collaboration_stage_execution_intent','collaboration_stage_execution_candidate_idx',false,'issue_id,candidate_id,admitted_at,intent_id',''),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_observation_id_idx',true,'observation_id',''),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_observation_native_idx',true,'intent_id,origin,native_id,native_version',''),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_terminal_idx',true,'intent_id','(result = ANY (ARRAY[''succeeded''::text, ''failed''::text, ''not_started''::text]))'),
  ('collaboration_stage_execution_observation','collaboration_stage_execution_observation_order_idx',false,'intent_id,created_at,observation_id',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_id_idx',true,'observation_id',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_version_idx',true,'stream_key,version',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_content_idx',true,'stream_key,native_version,observation_digest',''),
  ('collaboration_stage_rollback_observation','collaboration_stage_rollback_observation_order_idx',false,'issue_id,candidate_id,created_at,observation_id','')
 ) AS want(tbl,idx,uniq,cols,pred)
 WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass(want.tbl) AND c.relname=want.idx AND a.amname='btree'
 AND i.indisunique=want.uniq AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indexprs IS NULL AND i.indnkeyatts=i.indnatts
 AND (SELECT string_agg(pg_get_indexdef(i.indexrelid,k,true),',' ORDER BY k) FROM generate_series(1,i.indnkeyatts::int) k)=want.cols
 AND COALESCE(pg_get_expr(i.indpred,i.indrelid),'')=want.pred
 )) AND NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('collaboration_stage_execution_intent'),('collaboration_stage_execution_observation'),('collaboration_stage_rollback_observation')
 ) AS want(tbl) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 WHERE t.tgrelid=to_regclass(want.tbl) AND t.tgname=want.tbl||'_immutable' AND NOT t.tgisinternal
 AND t.tgenabled='O' AND t.tgtype=19 AND p.proname='collaboration_stage_immutable'
 ))) THEN
 RAISE EXCEPTION 'stage execution indexes or immutability triggers are unavailable' USING ERRCODE='23514';
 END IF;
END $$;
