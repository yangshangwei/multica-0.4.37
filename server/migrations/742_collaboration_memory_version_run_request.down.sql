DO $$ BEGIN
    RAISE EXCEPTION 'collaboration memory run idempotency cannot be reversed; retain the compatible binary and data';
END; $$;
