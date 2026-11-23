DO $$ BEGIN
    RAISE EXCEPTION 'collaboration run shared-context facts cannot be reversed; retain the compatible binary and data';
END; $$;
