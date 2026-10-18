DO $$ BEGIN
    RAISE EXCEPTION 'collaboration Switch proposal identity cannot be reversed';
END; $$;
