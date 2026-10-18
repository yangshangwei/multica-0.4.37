DO $$ BEGIN RAISE EXCEPTION 'collaboration leader order cannot be downgraded to transaction timestamps'; END $$;
