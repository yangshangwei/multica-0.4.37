# Research: Final runtime-material planning consistency review

- Query: Check PRD/design/contracts/implementation plan for blocking contradictions before selected-runtime/no-App implementation.
- Scope: internal, read-only design review
- Date: 2026-11-27

## Findings

Reviewed `prd.md`, `design.md`, `contracts.md` and `implement.md` after the main session consolidated the three research lanes. No remaining blocking inconsistency was identified in the reviewed contract after the clarifications below landed.

Three essential ambiguities were raised and addressed in the final `contracts.md`:

1. **Immutable operation versus mutable claim binding.** Task identity is preallocated with the operation in the enqueue transaction; expected descriptor digest excludes claim state. Reclaim supplies a fresh generation/host challenge. Stale claims cannot promote new observations. Original accepted receipt replay is checked before current operation-state validation but remains subject to current read authorization.
2. **Plan verification occurs before model launch.** Auto-off creates an issue without runnable task/preparation. An authorized plan task may report its exact `verify_baseline` observation while dispatched/preparing. A valid fresh preparation receipt is required at start; reclaim invalidates it. The model does not need to be running to prove its input readable, and missing input never launches `backend.Execute`.
3. **Daemon authentication is more than a credential label.** Existing `DaemonAuth` explicitly accepts owner PAT/JWT fallback (`server/internal/middleware/daemon_auth.go:62`) and can share a PAT with human CLI (`:70`). The contract now uses registered daemon/runtime/current-claim binding and a host-only challenge delivered through claim/finalization transport. Generic GET does not mint/reveal a challenge; a model `mat_` cannot submit trusted receipts. An authorized machine owner who can impersonate its daemon remains within the documented trusted-host boundary. No new managed-enrollment requirement was introduced.

The canonical capability name is `material-operations-v1`. Earlier research naming is nonauthoritative and should not create a second accepted protocol.

The main design preserves the required ownership and state boundaries:

- All remote writes/merges are performed by the selected runtime; host observer only reads and transports facts.
- Server has no material Git credential/network fallback, including plan-baseline preparation.
- Single explicit `execution_runtime_id` covers Git and plan stages; internal carriers are not another user selection or shared user-Agent rebind.
- Logical jobs and per-task attempts are distinct; queued/running/unknown outcomes block duplicate mutation even after scheduler lease expiry.
- Trusted observations carry complete changes and fixed-commit file bytes; model-authored JSON and ordinary task completion cannot establish a baseline.
- Existing manual holds, human decisions, original request recovery, terminal plan retry and historical evidence remain mandatory.
- Runtime deletion preserves audit history; old App settings require explicit runtime selection and do not silently migrate old in-flight authority.

## Related specs and evidence

- `.trellis/spec/server/git-materials.md`
- `.trellis/spec/server/task-api-capabilities.md`
- `.trellis/spec/server/security-boundaries.md`
- `server/internal/middleware/daemon_auth.go:62`
- `server/internal/service/material_plan.go:336` (current server-read dependency slated for replacement)
- `server/pkg/protocol/messages.go:157` (ordinary completion is not material proof)

## Caveats / Not Found

This is a planning consistency review, not implementation verification. No code, tests, database, real model or Git provider operation was executed. The acceptance checklist still requires real implementation tests for challenge secrecy/replay, prelaunch lifecycle ordering, retained attempts and no server credential path. Trust in the configured daemon does not prevent arbitrary shell operations using ambient host credentials; the documents explicitly retain that limitation.
