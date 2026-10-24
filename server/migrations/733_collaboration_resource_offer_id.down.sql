DO $$ BEGIN
    RAISE EXCEPTION 'collaboration resource offer identity cannot be reversed; retain the compatible binary and data';
END; $$;
