DO $$ BEGIN
    RAISE EXCEPTION 'collaboration Switch consent history cannot be reversed; retain the compatible binary and data';
END; $$;
