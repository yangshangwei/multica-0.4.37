DO $$ BEGIN
    RAISE EXCEPTION 'collaboration Switch proposals require the forward execution model';
END; $$;
