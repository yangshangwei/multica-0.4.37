DO $$ BEGIN
    RAISE EXCEPTION 'shared skill execution consent cannot be downgraded to the previous configuration model';
END $$;
