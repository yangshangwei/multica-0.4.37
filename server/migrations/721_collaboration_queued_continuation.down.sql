DO $$ BEGIN RAISE EXCEPTION 'collaboration continuation model cannot roll back to the old execution model'; END $$;
