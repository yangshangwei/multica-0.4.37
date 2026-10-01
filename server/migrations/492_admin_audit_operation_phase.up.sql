CREATE UNIQUE INDEX CONCURRENTLY admin_audit_operation_phase_uidx ON admin_audit_event (operation_id, phase) WHERE operation_id IS NOT NULL;
