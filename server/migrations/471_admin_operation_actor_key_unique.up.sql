CREATE UNIQUE INDEX CONCURRENTLY admin_operation_actor_key_uidx ON admin_operation (organization_id, actor_id, idempotency_key) WHERE actor_id IS NOT NULL;
