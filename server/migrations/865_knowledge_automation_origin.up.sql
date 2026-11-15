CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_automation_origin_idx ON collaboration_knowledge_candidate (automation_job_id, lineage_id) WHERE proposal_origin='automation';
