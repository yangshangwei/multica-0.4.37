DO $$ BEGIN
 IF NOT (SELECT NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('submission_id','uuid'),('issue_id','uuid'),('workspace_id','uuid'),
 ('lifecycle_epoch','bigint'),('policy_revision','bigint'),('role_revision','bigint'),('stage_revision','bigint'),
 ('scope_decision_id','uuid'),('candidate_id','uuid'),('candidate_version','bigint'),('content_digest','text'),
 ('release_decision_id','uuid'),('execution_observation_id','uuid'),('operation_id','uuid'),('actor_user_id','uuid'),('created_at','timestamp with time zone')
 ) AS want(col,typ) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=to_regclass('collaboration_stage_submission') AND a.attname=want.col
 AND a.attnotnull AND NOT a.attisdropped AND format_type(a.atttypid,a.atttypmod)=want.typ
 )) AND NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('collaboration_stage_submission_id_idx',true,'submission_id'),
 ('collaboration_stage_submission_issue_idx',false,'issue_id,lifecycle_epoch,created_at,submission_id')
 ) AS want(idx,uniq,cols) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
 WHERE i.indrelid=to_regclass('collaboration_stage_submission') AND c.relname=want.idx AND a.amname='btree'
 AND i.indisunique=want.uniq AND i.indisvalid AND i.indisready AND i.indislive
 AND NOT i.indisexclusion AND i.indexprs IS NULL AND i.indpred IS NULL AND i.indnkeyatts=i.indnatts
 AND (SELECT string_agg(pg_get_indexdef(i.indexrelid,k,true),',' ORDER BY k) FROM generate_series(1,i.indnkeyatts::int) k)=want.cols
 )) AND EXISTS (
 SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
 WHERE t.tgrelid=to_regclass('collaboration_stage_submission') AND t.tgname='collaboration_stage_submission_immutable' AND NOT t.tgisinternal
 AND t.tgenabled='O' AND t.tgtype=19 AND p.proname='collaboration_stage_immutable'
 ) AND NOT EXISTS (
 SELECT 1 FROM (VALUES
 ('CHECK ((lifecycle_epoch > 0))'),('CHECK ((policy_revision > 0))'),('CHECK ((role_revision > 0))'),
 ('CHECK ((stage_revision > 0))'),('CHECK ((candidate_version > 0))'),('CHECK ((content_digest ~ ''^[0-9a-f]{64}$''::text))')
 ) AS want(def) WHERE NOT EXISTS (
 SELECT 1 FROM pg_catalog.pg_constraint c WHERE c.conrelid=to_regclass('collaboration_stage_submission') AND c.contype='c' AND c.convalidated AND pg_get_constraintdef(c.oid)=want.def
 ))) THEN
 RAISE EXCEPTION 'stage submission schema is unavailable' USING ERRCODE='23514';
 END IF;
END $$;
