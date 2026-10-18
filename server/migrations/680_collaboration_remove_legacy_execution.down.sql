DO $$
BEGIN
    RAISE EXCEPTION 'removed unreleased collaboration execution cannot be reconstructed; restore an explicit pre-migration backup';
END;
$$;
