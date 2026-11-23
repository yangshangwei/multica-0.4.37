-- The cloud runtime waitlist form and its endpoint are gone, so nothing reads
-- or writes these columns. Dropping them discards any addresses collected
-- before; back up "user" first if they are still needed.
ALTER TABLE "user"
  DROP COLUMN IF EXISTS cloud_waitlist_email,
  DROP COLUMN IF EXISTS cloud_waitlist_reason;
