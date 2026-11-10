# Independent review: D6 batch client

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`. HEAD at review: `c9a070551`.

## Verdict

**PASS_CLIENT_PROTOCOL_AND_CACHE_SCOPE.** No blocking defect was found in the
13 frozen client/type/test files listed in `batch-client-implementation.md`.
All 13 current SHA-256 values match that inventory. The reviewer made no product
edit and did not touch concurrent server/schema/P1 work.

This accepts the reviewed frontend protocol, cache and presentation increment.
It does not establish backend per-item transactions, real authentication/tar/
state/receipt acceptance, completion enforcement or M1 completion.

## Findings (fixed)

None required during independent review. The producer's original protocol,
cache, view and typecheck failures remain in their numbered records; none was
removed or reclassified as a passing run.

## Findings (not fixed)

No unresolved defect within the reviewed client scope. The backend implementation
must still emit the frozen D6 results and enforce actual per-item completion
bindings. Main owns the matching spec update and the existing end-to-end test's
exact response expectation, with assertions on actual persisted item states.
These are integration obligations outside this frontend-only verdict.

## Source assessment

- The response boundary uses existing zod/`parseWithFallback` with a non-success
  fallback and redacted diagnostics. It checks safe counts/revisions, exact
  request order/index/ID, number of items, committed count and legal committed
  prefix followed by one rejection/unknown and unattempted suffix. Malformed,
  truncated and unknown-state data cannot resolve as full success.
- Legacy complete counts remain usable without inventing per-item outcomes.
  Short counts produce an incomplete error; explicit no-mutation zero remains a
  no-op. The mutation whitelist matches the existing backend fields, including
  nullable mutation fields and excluding completion/reason/control metadata.
- Per-item binding validation rejects shared `updates.completion`, invalid
  canonical UUID/revision/extra fields, duplicates and unrelated items before
  transport. Request IDs are copied before sending and used for later response
  correlation. No whole-batch key, write retry, substitute key or auto-resume
  was introduced; redirects remain errors in the existing transport.
- Non-2xx errors preserve ApiError status, raw body, retry metadata and dynamic
  stale-auth semantics. Known committed prefixes remain typed facts; malformed
  errors, network failures and unknown results remain uncertain. A 200 partial
  result is still rejected rather than producing a full-success notification.
- The mutation strips the new controls from optimistic Issue data and forwards
  optional per-item bindings unchanged. Failure rolls back only a cache object
  still equal to this mutation's own optimistic value, preserving a newer WS or
  refetch snapshot. It then invalidates the workspace's issue/inbox projections
  and requested issue timeline/attachment/task keys for authoritative reads.
  This covers list/myAll/flat/table/detail/children under `issueKeys.all(wsId)`;
  successful full batches retain the prior surgical behavior.
- Surface actions return the original result/refusal. Both toolbar placements
  use `updated`, omit success for zero and retain selection on error. Run-confirm
  already closes only after a resolved mutation; its added regression confirms
  partial/unknown failures keep the dialog open without a success toast.
- Tests put the protocol matrix in the pure node API helper layer and use a real
  QueryClient/QueryObserver for partial-refetch and newer-snapshot regressions.
  Views test the composed count/error/wiring behavior rather than duplicating
  the protocol matrix. No dependency or lockfile changes were included.

## Evidence review

The reviewer rehashed and parsed all 17 producer run records and original logs,
including the failed runs. Vitest results come from the final JSON
`assertionResults`, not the common launcher's zero Go-event counts.

| Retained final evidence | Result |
| --- | --- |
| `client-contract-final02` | 63 passed: 33 API protocol/request + 30 mutation; 0 failed/excluded |
| `client-views-green01` | 91 passed; 0 failed/excluded |
| final core and corrected views typecheck | exit 0 |
| final changed-core lint | exit 0, no diagnostics |
| full views lint | exit 0, 27 warnings in unchanged, unowned files |

All test inventories have unique names and agree with reported totals. The
focused cache red keeps its 27 excluded tests separate from its two behavioral
failures. Initial two missing-mock failures and the later noImplicitReturns
errors are correctly recorded as authoring/type failures, not behavioral red.
The later strengthened cache red proves both no-refetch and newer-value-overwrite
failures. The five views red failures match result propagation and count/no-op
presentation defects.

The final core test record binds all 13 current client files. Owned Views files
are unchanged from their passing run; subsequent core changes were separately
tested and the reviewer typechecked both packages against the final current
sources. No old passing run is presented as testing newer bytes. Broad launcher
source snapshots include concurrent server edits; those are distinguished from
the stable 13-file client scope.

## Verification performed by reviewer

- `client-review-core-typecheck01`: full core typecheck, pass.
- `client-review-views-typecheck01`: full views typecheck against current core,
  pass.
- `client-review-core-lint01`: six changed core files, pass with no diagnostics.
- `client-review-views-lint01`: seven changed views files, pass with no diagnostics.
- Owned `git diff --check`: pass. Frozen source hashes and JSON/log bindings
  checked again after verification.
- Existing 63/91 meaningful tests were inspected and their original logs checked;
  they were not rerun without a new failure or code change.

All four reviewer commands used the task's existing private launcher with unique
run IDs, `corepack pnpm`, external execution state and no selected database.
No database/container/provider/browser action, product edit, commit, deployment,
push or external message was performed by this reviewer.

Two read-only lookups guessed absent `issues/keys.ts` and `move-mutation.ts`;
`rg --files` located the actual keys and move path in `queries.ts` and
`mutations.ts`. These are recorded lookup failures, not product red or tests.

Exact reviewed hashes, all 21 producer/reviewer record and log identities,
per-file test counts, scope drift and limits are in
`batch-client-independent-review.json`. Later product changes require review of
their delta and appropriate evidence; this report must not close the backend
gate, the enforcement task or M1.
