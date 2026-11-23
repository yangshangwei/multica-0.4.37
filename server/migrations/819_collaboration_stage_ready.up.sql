DO $$ BEGIN
 IF NOT (SELECT NOT EXISTS (
 SELECT 1 FROM (VALUES
  ('project_stage_policy','project_stage_policy_revision_idx',true,'workspace_id,project_id,revision',''),
  ('project_stage_policy','project_stage_policy_request_idx',true,'workspace_id,project_id,created_by,request_id',''),
  ('collaboration_stage_task','collaboration_stage_task_issue_idx',true,'issue_id',''),
  ('collaboration_stage_role','collaboration_stage_role_revision_idx',true,'issue_id,revision',''),
  ('collaboration_stage_candidate','collaboration_stage_candidate_id_idx',true,'candidate_id',''),
  ('collaboration_stage_candidate','collaboration_stage_candidate_version_idx',true,'issue_id,proposal_id,version',''),
  ('collaboration_stage_decision','collaboration_stage_decision_id_idx',true,'decision_id',''),
  ('collaboration_stage_decision','collaboration_stage_decision_issue_idx',false,'issue_id,created_at,decision_id',''),
  ('collaboration_stage_decision','collaboration_stage_decision_candidate_idx',true,'candidate_id','(step = ''proposal''::text)'),
  ('collaboration_stage_decision','collaboration_stage_decision_accept_idx',true,'proposal_id','((step = ''proposal''::text) AND (decision = ''accepted''::text))'),
  ('collaboration_stage_child','collaboration_stage_child_issue_idx',true,'child_issue_id',''),
  ('collaboration_stage_child','collaboration_stage_child_root_idx',false,'root_issue_id,child_issue_id',''),
  ('collaboration_stage_child','collaboration_stage_child_materialization_idx',true,'proposal_id,ordinal','(origin = ''materialized''::text)')
 ) AS want(tbl, idx, uniq, cols, pred)
 WHERE NOT EXISTS (
  SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_am a ON a.oid=c.relam
  WHERE i.indrelid=to_regclass(want.tbl) AND c.relname=want.idx AND a.amname='btree'
   AND i.indisunique=want.uniq AND i.indisvalid AND i.indisready AND i.indislive
   AND NOT i.indisexclusion AND i.indexprs IS NULL AND i.indnkeyatts=i.indnatts
   AND (SELECT string_agg(pg_get_indexdef(i.indexrelid,k,true), ',' ORDER BY k) FROM generate_series(1,i.indnkeyatts::int) k)=want.cols
   AND COALESCE(pg_get_expr(i.indpred,i.indrelid),'')=want.pred
 )
) AND NOT EXISTS (
 SELECT 1 FROM (VALUES ('project_stage_policy','project_stage_policy_immutable'),('collaboration_stage_role','collaboration_stage_role_immutable'),('collaboration_stage_candidate','collaboration_stage_candidate_immutable'),('collaboration_stage_decision','collaboration_stage_decision_immutable')) AS want(tbl, trg)
 WHERE NOT EXISTS (
  SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid
  WHERE t.tgrelid=to_regclass(want.tbl) AND t.tgname=want.trg AND NOT t.tgisinternal AND t.tgenabled='O'
   AND p.proname='collaboration_stage_immutable'
 )
)) THEN RAISE EXCEPTION 'collaboration stage schema unavailable'; END IF;
END; $$;
