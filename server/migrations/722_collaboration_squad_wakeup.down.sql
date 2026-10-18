DO $$ BEGIN RAISE EXCEPTION 'collaboration continuation intent cannot roll back to the old execution model'; END $$;
