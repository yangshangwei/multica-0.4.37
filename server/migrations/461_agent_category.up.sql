ALTER TABLE agent
    ADD COLUMN category TEXT NOT NULL DEFAULT ''
        CONSTRAINT agent_category_length CHECK (char_length(category) <= 50);

COMMENT ON COLUMN agent.category IS
    'User-defined directory category. Empty means uncategorized; independent of role-template provenance and autonomy.';
