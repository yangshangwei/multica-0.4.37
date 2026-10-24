DO $$ BEGIN
    RAISE EXCEPTION 'collaboration resource offer history cannot be reversed; retain the compatible binary and data';
END; $$;
