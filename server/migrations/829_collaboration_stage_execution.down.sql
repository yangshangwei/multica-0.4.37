-- Admission must be stopped for the complete multi-file downgrade. Refuse
-- before removing any supporting index when immutable history exists.
DO $$ BEGIN
 LOCK TABLE collaboration_stage_execution_intent, collaboration_stage_execution_observation,
 collaboration_stage_rollback_observation IN ACCESS EXCLUSIVE MODE NOWAIT;
 IF EXISTS (SELECT 1 FROM collaboration_stage_execution_intent)
 OR EXISTS (SELECT 1 FROM collaboration_stage_execution_observation)
 OR EXISTS (SELECT 1 FROM collaboration_stage_rollback_observation) THEN
 RAISE EXCEPTION 'stage execution history is retained' USING ERRCODE='23514';
 END IF;
END $$;
DROP TABLE IF EXISTS collaboration_stage_rollback_observation;
DROP TABLE IF EXISTS collaboration_stage_execution_observation;
DROP TABLE IF EXISTS collaboration_stage_execution_intent;
