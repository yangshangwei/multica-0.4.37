DO $$ BEGIN RAISE EXCEPTION 'collaboration frozen dependency model cannot roll back to the old execution model'; END $$;
