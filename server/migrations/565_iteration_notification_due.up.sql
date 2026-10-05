CREATE INDEX CONCURRENTLY iteration_notification_due ON iteration_notification (next_attempt_at) WHERE status = 'pending';
