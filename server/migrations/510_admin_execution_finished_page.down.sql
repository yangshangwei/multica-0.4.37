-- Maintenance only: refuse active writers before checking retained evidence.
DO $$
BEGIN
    LOCK TABLE admin_alert, agent_task_queue IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM admin_alert) THEN
        RAISE EXCEPTION 'administrative alert data exists; preserve overview drilldown support';
    END IF;
    DROP INDEX admin_execution_finished_page_idx;
END;
$$;
