CREATE UNIQUE INDEX CONCURRENTLY agent_artifact_execution_idempotency_idx ON agent_artifact_execution (workspace_id, initiator_kind, initiator_id, idempotency_key);
