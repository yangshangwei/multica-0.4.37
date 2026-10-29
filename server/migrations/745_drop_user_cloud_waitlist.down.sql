-- Restores the columns only; addresses dropped by the up migration are lost.
ALTER TABLE "user"
  ADD COLUMN IF NOT EXISTS cloud_waitlist_email VARCHAR(254),
  ADD COLUMN IF NOT EXISTS cloud_waitlist_reason TEXT;
