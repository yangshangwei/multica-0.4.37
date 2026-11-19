DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM (VALUES
            ('unblock_condition', 'text'),
            ('estimated_resolution_at', 'timestamp with time zone'),
            ('superseded_by_repair_handoff_id', 'uuid')
        ) AS want(col, typ) WHERE NOT EXISTS (
            SELECT 1 FROM pg_catalog.pg_attribute a
            WHERE a.attrelid = to_regclass('issue_handoff') AND a.attname = want.col
                AND NOT a.attnotnull AND NOT a.attisdropped
                AND format_type(a.atttypid, a.atttypmod) = want.typ
        )
    ) THEN
        RAISE EXCEPTION 'information recovery schema is unavailable' USING ERRCODE = '23514';
    END IF;
END $$;
