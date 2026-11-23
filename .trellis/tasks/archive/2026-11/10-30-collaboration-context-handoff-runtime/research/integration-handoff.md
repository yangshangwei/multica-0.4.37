# Runtime implementation handoff — preparing final source

Coordinator: Codex session `01a191c6-bf9b-74b3-b80e-8258a46b4b5b`.
Final integration/parent owner: `01a1920d-a340-7dd3-9d25-9191aa128cb0`, isolated worktree `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`.

This is a live ownership record. Shared product files are not yet released; the final source manifest will replace this notice once the remaining fixes are verified. Do not infer final acceptance from intermediate test counts.

## Ownership

- `checkpoint_implementation`: protocol/recording checkpoints and seal, source-owned records, host recording/sender/ledger recovery and related tests.
- `handoff_implementation`: handoff envelope, snapshots/source reads, complete wakeup provenance, squad hooks, current host_runner wiring and corresponding service tests.
- `recall_delivery`: human/runtime comment recall/content chunks, CLI, core schemas/queries, shared comment/retention/handoff UI and built-in guidance.
- Main runtime coordinator: explicit claim query negotiation and version validation, `server/cmd/server/router.go`, `collaboration_runtime_capabilities.go` outcome/proof hooks, canonical handler credential-boundary coverage.
- Parent integration owner: external launch-injection patch reconciliation, independent full checks, new Web/Desktop acceptance files, final spec/docs/commits and parent bookkeeping. Its coordination artifacts in this task are preserved.

## Root integration already wired

- New host claim URL requests `handoff_version=1&recording_version=1`; old body and old clients remain unchanged. Optional response fields activate only requested supported host features; unknown/namespace feature versions fail closed before mutation/launch.
- Nine new runtime/human routes wire checkpoint/seal, live/snapshot/bind/source handoff, comments/content and human run handoff.
- Outcome handling redacts the new recording format and validates accepted checkpoint prefix before inserting outcome. Member completion records a pending proof wake cause before scheduling continuation.
- `make sqlc` has generated the current handoff/recall/records queries; checkpoint owner may still change the bounded seal-event query and will request final regeneration.

Root checks executed: daemon optional-version/legacy/namespace negotiation suite passed; handler invalid-version matrix passed (1 top-level + 4 subtests); canonical purpose-boundary test passed across old/new handlers. Logs available under `research/verification/` (daemon result was returned by the execution tool, not yet a durable full log).

## Remaining fixes before source release

1. Recording recovery: wire original pending checkpoint replay and metadata seal into normal `recoverHost`, including handoff refusal/fallback failures after provider activity. Parent isolated combined regression is red until these calls exist.
2. Recording seal: bounded monotonic later metadata with exact original-key replay, separate from new content; final SQL bound must match maximum checkpoint+seal+outcome events.
3. Handoff budget: preserve a legal 64 KiB admitted assignment containing JSON HTML-escaped characters without violating actual 256 KiB envelope bound.
4. Frontend: fixed-watermark retention consistency across the full page chain; new handoff query access/legacy/stale behavior; source-run navigation preserving issue selection; human-readable current handler identity.

The backpressure fixture's explicit release fix is already applied; a fixture cleanup hang is not evidence of a product deadlock. The parent owns the stronger launch patch in its isolated tree only; do not apply it a second time in main or replace shared files wholesale.

## Evidence and environments

- Entry gate and 79 verified predecessor hashes: `entry-verification.md`.
- Recall lane first pass: `recall-implementation.md`; its counts predate the final follow-up fixes.
- Launch lane owned-only patch and checks: `launch-injection-handoff.md`; final integration must reconcile with this tree's checkpoint hooks.
- Four task-owned DBs and private env-file paths: `verification/environment.json`. They are separate from the parent's integration DB and browser environment. No real Provider calls are being made by this implementation session.
- Original E-series state is parent-owned. New runtime tests cannot retroactively replace its source-bound trials or claim model comprehension.
