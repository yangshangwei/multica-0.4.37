CREATE UNIQUE INDEX CONCURRENTLY iteration_notification_day ON iteration_notification (iteration_id, recipient_user_id, local_date, kind) WHERE local_date IS NOT NULL;
