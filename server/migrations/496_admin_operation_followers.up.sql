CREATE INDEX CONCURRENTLY admin_operation_followers_idx ON admin_operation (root_operation_id,state,id) WHERE root_operation_id IS NOT NULL;
