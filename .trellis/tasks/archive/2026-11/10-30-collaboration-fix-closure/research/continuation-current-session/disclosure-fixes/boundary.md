# Disclosure-only E2E maintenance

The committed detail view keeps Attachments, Sub-issues, Knowledge candidates
and empty AI execution behind native details summaries. Existing journeys still
expect those ordinary entry points to be expanded when entering an issue.

This slice adds one small shared E2E helper that clicks an exact direct summary
only when its details is closed. It updates the assigned desktop/delivery/records,
knowledge and resource journeys at actual fresh-entry or navigation boundaries.
The helper does not infer authority or select a fallback action.

All original file bytes, download/native IPC checks, source/privacy/UUID checks,
WS revocation, resource consent/history and knowledge version/decision assertions
remain. Existing run anchors and pending recovery must still reveal themselves;
the corresponding assertions receive no manual-open assistance.

No product or frozen v12 edits, services, database, Provider calls, widened
timeouts, skips, retries, forced clicks or DOM-open writes are in scope. Root owns
real browser execution. This slice runs TypeScript, lint/static and discovery.
