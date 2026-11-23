DO $$ BEGIN
    RAISE EXCEPTION 'collaboration Switch consent identity cannot be reversed';
END; $$;
