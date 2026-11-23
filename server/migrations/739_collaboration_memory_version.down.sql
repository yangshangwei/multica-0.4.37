DO $$ BEGIN
    RAISE EXCEPTION 'collaboration memory history cannot be reversed; retain the compatible binary and data';
END; $$;
