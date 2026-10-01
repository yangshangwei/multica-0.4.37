DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM admin_alert) THEN
        RAISE EXCEPTION 'administrative alert data exists; preserve overview drilldown support';
    END IF;
END $$;
DROP INDEX admin_execution_finished_page_idx;
