CREATE UNIQUE INDEX CONCURRENTLY delivery_evaluation_key_idx ON delivery_evidence_evaluation (workspace_id, evaluator_user_id, request_key);
