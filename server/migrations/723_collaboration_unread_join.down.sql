DO $$ BEGIN RAISE EXCEPTION 'collaboration unread joins cannot be downgraded to the obsolete execution model'; END $$;
