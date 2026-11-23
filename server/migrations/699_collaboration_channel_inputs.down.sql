-- Old binaries cannot interpret a returned input or immutable group membership.
-- Refuse reversal rather than erase queue positions or historical consumption.
DO $$
BEGIN
    RAISE EXCEPTION 'collaboration channel input migration cannot be reversed; retain the compatible binary and data';
END;
$$;
