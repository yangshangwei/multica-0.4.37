# Design

The primary root cause is an unapplied local rollout: the running desktop consumes API :18572 from pre-rename commit cfae7477e and DB migration 456. Upgrade that exact environment after backup and validation, retaining existing profile/data/ports. Do not use directory-only environment lookup, which selects a separate test environment.

Add migration 458 without altering shipped 457. Only renamed builtins with exactly two untouched product onboarding rows and no queued/finished tasks qualify. Match whole deterministic legacy English/Chinese opening, replace only the name in its self introduction, and update its exact quoted opening in the hidden kickoff. Lock the session before rechecking, so first-send cannot race. Preserve IDs, timestamps, profile data and workspace text, including literal Mika in workspace names. Historical real conversations remain unchanged. Down is a documented no-op to avoid restoring obsolete branding or undoing user work.

Current shared views render saved agent names/descriptions correctly; no frontend masking of old data. Audit Web/Desktop/Mobile/docs and preserve internal API/system identifiers. Capture actual Electron before/after and verify current API source and DB ledger, not only source tests.

The audit found eight product-owned display references in the Web landing changelog (four per language). Update that editorial branding too; release dates, behavior descriptions, internal identifiers and user-authored history remain intact.
