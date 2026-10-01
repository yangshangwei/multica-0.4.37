CREATE INDEX CONCURRENTLY admin_operation_installation_time_idx ON admin_operation (organization_id,target_installation_id,accepted_at DESC,id DESC) WHERE target_installation_id IS NOT NULL;
