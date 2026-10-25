DO $$
BEGIN
    RAISE EXCEPTION 'host execution is forward-only; disable host admission while retaining launch identities and receipts';
END $$;
