DO $$ BEGIN
    RAISE EXCEPTION 'collaboration memory version order cannot be reversed; retain the compatible binary and data';
END; $$;
