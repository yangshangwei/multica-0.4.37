# Startup injection lane — 2026-10-31

The user explicitly selected **补齐启动注入与验证** after noting parallel service work. This lane does not take ownership of checkpoint, recall, protocol or server snapshot implementation.

- Owner: Codex session `01a191c6-6a2a-7313-b674-485c195a5cae`, native agent `launch_injection`.
- Isolated worktree: `/Volumes/artisan/code/2026/mc-handoff-launch-20261031`.
- Code ownership: `server/internal/daemon/collaborationrun/host_runner.go`, optional `host_ledger.go`, new `host_handoff.go`, `client_handoff.go`, focused launch/client/ledger tests.
- Borrowed prerequisites remain owned by the parallel flow: handoff DTO, claim negotiation, server snapshot/binding, checkpoints, generated SQL and router. Captured prerequisite hashes are in the isolated worktree's `research/launch-injection-baseline.json`.
- Contract: negotiated snapshot fetched before Execute; admitted inputs remain intact; handoff source text is bounded data with provenance; actual provider attempt binds snapshot/hash after authorize and before process start. Preparation launches do not bind. Failure cannot silently downgrade or skip original Stop/proof cleanup.
- Validation: fake provider actual stdin, stable snapshot over retries/fallback, malformed/unauthorized/bind failures unlaunched, original proof receipts, no handoff text in ledger/brief. No paid provider calls in this lane.
- Integration: apply only this lane's changes and preserve concurrent edits. If shared host files change in the other flow, reconcile the patch against their latest content rather than replacing whole files.

## Observed integration contract

- Claim negotiation: `handoff_version=1` URL query; response `HandoffVersion`, absent/0 means the legacy path.
- Snapshot: `POST .../runs/{runId}/handoff/snapshots`, request `{request_id,memory_version}`.
- Bind: `POST .../runs/{runId}/handoff/bind`, request `{snapshot_id,prompt_sha256,attempt_seq,attempt_nonce}`; strict receipt must echo the exact identity and hash.
- The parallel flow has added an initial `host_handoff.go` and runner wiring. This lane now tests and fixes that borrowed implementation, preserving checkpoint hooks.
- Known validation targets: stable snapshot ID across outer fresh fallback; exact input hash when the Codex adapter internally resumes/falls back; original claim/assignment and memory consistency; strict smoke query allowlist.

Status: implementation started; not a completion claim.
