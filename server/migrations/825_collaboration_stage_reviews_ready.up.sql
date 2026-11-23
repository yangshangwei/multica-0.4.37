-- Apply after the three concurrent indexes. It is also the first rollback
-- boundary, so retained history refuses before any supporting index is lost.
DO $$ BEGIN
 IF (SELECT count(*) FROM pg_catalog.pg_index idx
 JOIN pg_catalog.pg_class cls ON cls.oid=idx.indexrelid
 WHERE cls.relnamespace=(SELECT relnamespace FROM pg_catalog.pg_class WHERE oid='collaboration_stage_candidate'::regclass)
 AND cls.relname IN ('collaboration_stage_code_version_idx','collaboration_stage_revocation_decision_idx','collaboration_stage_revocation_id_idx')
 AND idx.indisvalid AND idx.indisready AND idx.indislive AND idx.indisunique) <> 3 THEN
  RAISE EXCEPTION 'stage review indexes are unavailable' USING ERRCODE='23514';
 END IF;
END $$;
