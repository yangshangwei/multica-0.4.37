DO $$ BEGIN
    RAISE EXCEPTION 'collaboration memory identity cannot be reversed; retain the compatible binary and data';
END; $$;
