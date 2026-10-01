# Platform observability contracts

These rules describe the implemented S06 read models, detector, audit and UI boundary. See `server/internal/handler/admin_overview.go`, `server/internal/service/admin_alert.go`, and `packages/core/admin/observability-schema.ts`.

- Finished outcomes and usage use `completed_at`; drilldowns preserve the returned window, timezone and `time_basis=finished`. A zero success-rate denominator is unknown/no samples, never0%. Missing usage remains unknown; token counts are not a bill.
- All-age active work requires `state_scope=current`, an explicit nonterminal status and created basis with no explicit time window. Do not turn this into an unbounded historical feed.
- Queue clocks distinguish actual transitions from migration observations. Existing queued rows receive an observation timestamp, not invented queue-entry history. Exact queue percentiles exclude observed/unknown clocks.
- Alert handling and source condition are independent. An execution failure is a historical fact, with permanent per-attempt dedupe even after closure. Inactive alert windows use `first_seen_at`; active alerts span all ages.
- Each detector lease advances bounded batches through a fixed cycle horizon, then rescans. Moving horizons can starve tail rows; one-way cursors can lose late commits. Event hints are bounded and nonblocking.
- Source failures pause opening and recovery. An absent organization-scoped row does not prove global deletion. Only a successful global absence check can resolve a deleted queued task as `task_removed`.
- Health reports real query/worker observations. Never-run is unknown; stale progress is explicit. A failing first cycle must remain visible even if other rules have never run.
- New audit events snapshot actor names; missing historical names remain unknown. Project only controlled fields/codes. A nullable `resolution_code` is a legitimate unresolved alert snapshot and must be accepted by the core schema without accepting arbitrary codes.
- Alert writes reuse current human authority locks, expected versions, original idempotency keys, durable failed receipts and atomic audit. Eligible assignees are active complete-password superadmins. Do not duplicate the S04 operation recovery state machine in alert UI.
- Retention settings declare configuration only. Automatic deletion remains disabled until independently approved and implemented.

Regressions live beside the handlers/service and core schemas. Browser acceptance covers navigation, audit decoding, lost-response recovery and mobile geometry. Run handler test processes serially because their TestMain owns fixed shared fixtures.
