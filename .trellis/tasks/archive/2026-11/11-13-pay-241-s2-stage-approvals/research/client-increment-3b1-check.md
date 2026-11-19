# S2 client3b1 — independent frozen-snapshot check

Completed: 2026-11-14T07:20:50.275108+00:00

**No new findings in this bounded increment.** Client3b1 refusal feedback is ready for integration with the continuing client work. This is not S2, A01–A12, or parent acceptance.

## Source boundary

Reviewed only the product source in `/Users/artisan/.multica/acceptance/pay241-20261113-01/snapshots/client-3b1-20261114T062238Z`. All **482/482 snapshot files**, **24/24 changed files**, **4/4 protected files**, and **19/19 evidence logs** match their recorded hashes. Every log’s embedded exit matches its declared result. The manifest SHA-256 is `c0ff1fe928d7597f61ba6c548e031f207f1c9b5b24579ebfd8145156f9d3cf02`. The result report SHA-256 remains `0fd3706d190fe2fb2ef7da5d89ec77186f93be02c4cf933a043ebaca98578faa`.

The current client3b2 files were not used as frozen3b1 source. The original snapshot/manifest, product/tests/specs/contract and prior reports were not edited. Git was used only to read the recorded commit for missing pure dependencies and contract references.

## Findings (fixed)

None; this was a read-only review.

## Findings (not fixed)

No new client3b1 defects found. Two pre-existing ProjectDetail hook warnings remain outside this increment; the recorded HEAD-to-snapshot diff changes only the API import and DELETE error callback.

## Reviewed paths and proof

### G1 — Strict stored stage refusal fields

No issue. Generic stage_incomplete requires 409, nonempty bounded blockers and a truncation flag. Hidden evidence identifiers, contradictory confirmations and stage blocks on policy-exit/non-stage outcomes are rejected. Unknown blocker codes remain blocking.

Snapshot sources: `packages/core/collaboration/schema.ts:351`, `packages/core/collaboration/stage-schema.ts:554`.

### G2 — Malformed is not missing; legacy 404 remains unchanged

No issue. A malformed stage-marked GET body throws instead of returning null. The unchanged generic recoverCollaborationAction still resends the frozen original on genuine 404/missing legacy receipt. No automatic new key/payload is introduced.

Snapshot sources: `packages/core/api/client.ts:1689`, `packages/core/collaboration/action.ts:94`, `packages/core/collaboration/result-operation.test.ts:204`.

### G3 — Result and source-form retention

No issue. Valid stored refusal retires only the operation draft; delivery form completion cleanup requires a successful status. Refusal does not dismiss the form. Recovery restores its original action/input, while malformed responses retain pending identity and input. Existing scope/access guards remain in the actual path.

Snapshot sources: `packages/core/collaboration/operation.ts:36`, `packages/core/collaboration/operation.ts:62`, `packages/core/collaboration/result-operation.test.ts:217`, `packages/views/collaboration/workflow.tsx:54`, `packages/views/collaboration/workflow.tsx:97`.

### G4 — R38 diagnostic error vs stored success

No issue. The actual admin error envelope is code/error/request_id, where request_id is a transport trace. Stage refusal parser strips prose/trace and does not manufacture an operation receipt. A first known rejected attempt removes its draft; a later rejected attempt with an existing unknown original retains that original and checks GET. GET results remain success-receipt-only; malformed failure-like results stay unresolved.

Snapshot sources: `packages/core/admin/collaboration-schema.ts:64`, `packages/core/admin/collaboration-schema.ts:140`, `packages/core/admin/collaboration-queries.ts:142`, `packages/views/admin/collaboration/collaboration-controls.tsx:42`.

### G5 — Shared status/blocker feedback

No issue. Views render retained HTTP status and historical blocker snapshot, distinguish a refused retry with unknown original from a known refusal, and preserve the reason. A stage refusal cannot render a completed receipt or raw private server error.

Snapshot sources: `packages/views/collaboration/stage-refusal-notice.tsx:6`, `packages/views/admin/collaboration/collaboration-receipt.tsx:7`, `packages/views/admin/collaboration/collaboration-controls.tsx:59`.

### G6 — Project DELETE409 stays put

No issue. Only current-session 409 stage_policy_exit_required gets the localized disable-policy instruction. Success toast/navigation remain in onSuccess. The recorded-HEAD useDeleteProject mutation removes data only on success; a failed delete preserves state. No delete-impact fields were invented.

Snapshot sources: `packages/views/projects/components/project-detail.tsx:264`, `packages/views/projects/components/project-detail.test.tsx:712`.

## Verification

**Independent execution:** three canonical pure schema suites from the frozen snapshot passed **173 tests**, zero failures/skips. The tests were bundled from exactly **20 snapshot sources plus three pure dependencies** fetched by `git show 8e824b303…:<path>`:

- `packages/core/paths/reserved-slugs.ts`
- `packages/core/types/property.ts`
- `packages/core/logger.ts`

All three supplements also match current bytes, but their provenance is the recorded commit. They live in a separate private supplement; no snapshot file was changed. The installed packages were Vitest4.1.0, Zod4.3.6 and esbuild0.25.12 under Node22.23.2. The current lockfile hash equals the recorded commit’s lockfile hash. Source hashes, external package versions/package hashes, generated bundle hashes, runner and log are bound in the JSON report and `dependency-binding.json`. No live3b2 implementation module was loaded.

**Historical checks verified, not rerun:** Core **365/365**, Views **136/136**, Core/Views typecheck pass, lint **0 errors / 2 pre-existing warnings**. The partial snapshot does not contain a full historical dependency/configuration tree, so these full-package checks remain recorded evidence. The independent esbuild bundling is not presented as a typecheck.

**Read-only backend references:** recorded-HEAD `adminError` emits `{code,error,request_id}`; administrative lifecycle defers rollback and operation GET emits only the existing success envelope. Recorded-HEAD `useDeleteProject` removes cached project data only in `onSuccess`. These references and their hashes are preserved separately; no server runtime behavior was claimed.

## Remaining integration boundary

Privacy/T4 changes, the eventual M1 completion consumer and activity projection are outside client3b1. Server2d must freeze any new R38 persisted-failure receipt before a client accepts one. Delete-impact metadata remains unimplemented rather than guessed. Final combined-client checks and real API/DB/production Web/Desktop acceptance remain open.

Only `research/client-increment-3b1-check.md` and `.json` were written in the repository. No API, DB, browser, model, platform/runtime service, full build or Git mutation was run.
