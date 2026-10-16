# S2 operation reads, revocation and explicit-write compatibility

Status: implementation under verification; FG is not passed. No capability,
enable, lifecycle, closure or client UI is enabled by this slice.

## Changes

- Authorized `GET /api/workspaces/{id}/iteration-operations/{requestID}` reads
  only the current actor's stored result, without accepting a client payload
  hash. It survives entity deletion and returns no-store responses. Workspace,
  subscriber and current-member locks remain held through the read transaction;
  an explicit RC statement prevents a pre-wait snapshot from restoring access.
- Ordinary create/update/batch and Plugin PATCH retain omitted fields, but reject
  explicit current_iteration_id or rollover counters with 428, including null
  and case variants. This is a temporary product restriction until confirmed
  lifecycle writes exist, not an implemented membership API.
- Member revocation takes the workspace fence before subscriber cleanup and
  removes I1 outbox plus matching protected inbox rows in the same transaction.
  Other workspaces/recipients and durable operation audit records remain intact.
- Built-in issue guidance and its source map describe these contracts.

## Behavioral evidence

- `/tmp/i1-operation-read-red.jsonl`: persisted deleted result initially 404,
  expected 200. The first green attempt compiled during another owner's edits;
  the second exposed a test-only nested withURLParam that replaced workspace
  route context. The corrected fixture and production owner pass the tests.
- `/tmp/i1-revoke-red.jsonl`: real member removal retained iteration_notification.
- `/tmp/i1-explicit-fields-red.jsonl`: update/batch returned 200 despite explicit
  iteration intent. Initial create cases hit fixture counter collisions and are
  not behavioral RED. Corrected `/tmp/i1-create-explicit-red2.jsonl` proves a
  create returned 201 and subsequent duplicate 409 instead of the required 428.
- `/tmp/i1-plugin-explicit-red.jsonl`: public PATCH returned 200 and changed title
  while silently ignoring explicit iteration intent.
- Final focused root boundary run `/tmp/i1-foundation-boundaries-final2.jsonl`:
  23 entries passed, zero failures/skips, with race detection. Includes rollback
  when final member deletion fails after notification cleanup.
- Independent read/revoke barrier tests: `/tmp/i1-operation-read-races-final.json`,
  6 entries passed (4 top-level), no skips. Both read-first and revoke-first
  assert actual PostgreSQL blocking, and a RR-starting fixture proves the owner
  chooses RC. Full JWT route tests `/tmp/i1-operation-route-tests.json`, 2 passed:
  URL workspace isolation and forged user-header rejection.

All DB tests use task-only local databases; default agent CLIs are guarded.
Final combined checks and remaining FG requirements are recorded by the parent
handoff after integration. No full repository Go, browser/Electron E2E or remote
CI pass is inferred from these focused results.

## Integration checks during this continuation

- Root combined handler regression passed 241 entries without failures/skips,
  with `-race`: `/tmp/i1-s2-integrated-handler.jsonl`. This precedes the final
  W07 principal-lock and stored-result validation refinements; final evidence
  for those must be recorded separately, not inferred from this pass.
- `pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1
  --force`: 15/15 tasks passed (`/tmp/i1-s2-resume-ts-static.log`).
- `pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force --
  --maxWorkers=2`: 5/5 tasks, 10,054 tests passed, no cache hits. Core2635,
  docs62, views6118, desktop957, web282; `/tmp/i1-s2-resume-ts-tests.log`.
  These root scripts exclude Mobile and do not prove browser/Electron E2E.
- `make sqlc` exited0 and all81 generated SHA256 values were unchanged:
  `/tmp/i1-resume-final-sqlc.log`, `/tmp/i1-resume-generated-before.json`.
- Existing ordinary-write regression benchmark (same S0/S1 fixture and command,
  300 writes x 3 runs each for1/4 writers): single453.1/432.4/494.0 writes/s,
  P95 3.461/3.769/2.974ms; four602.0/645.5/670.9 writes/s,
  P95 10.11/8.445/8.578ms. Medians pass frozen throughput and P95 gates;
  each run P95 remains below20ms. Eight transaction queries/write. Median lock
  statement span0.8836ms(single)/4.448ms(four); these spans include network,
  not solely database lock waits. `/tmp/i1-s2-resume-performance.log`.
  Populated workspace capacity/server lock-wait evidence is still pending FG.

Independent review found one stored-result validation gap before finalization:
malformed iteration/snapshot UUIDs, duplicate iteration IDs and counters beyond
JavaScript's safe range were accepted. The existing decoder now rejects these,
without querying live entities. `/tmp/i1-operation-validation-red.log` proves
RED; full domain suite `/tmp/i1-operation-validation-green.jsonl` passed88
entries (31 top-level), zero skips/failures, including18 boundary/control cases.

## Final combined verification

- Handler:395 pass,0 fail,2 existing opt-in skips (`TestTriageQueueScaleBaseline`
  and `TestTriageWireFixtures`), with race detection. This final selection covers
  iteration writers, creation/current authorization, T1, lifecycle transactions,
  onboarding, P1 association/deletion, protected operation reads and compatibility:
  `/tmp/i1-s2-final-handler.jsonl`.
- Service:167 pass,0 fail/skip with race detection across execution start, T1
  admission, failure reset, creation, quotas and project/Autopilot boundaries:
  `/tmp/i1-s2-integrated-service.jsonl`.
- Iteration + public API domain tests:93 pass,0 fail/skip with race detection:
  `/tmp/i1-s2-integrated-domain.jsonl`.
- Runtime/channel selection:79 pass,0 fail/skip with race detection:
  `/tmp/i1-s2-integrated-runtime-channel.jsonl`. Built-in guidance checks:22
  pass,0 fail/skip, `/tmp/i1-s2-builtins.jsonl`.
- Full Go build and vet exited0, logs `/tmp/i1-s2-resume-final-build.log` and
  `/tmp/i1-s2-resume-final-vet.log`. Context manifests and diff checks pass.

Commands use `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test`
with `-race -p 2 -parallel 2 -count=1 -json` (runtime/channel uses `-p 1` to avoid
shared fixture collisions). Every DB-backed selection names a local task-only
DATABASE_URL; the pure built-in test selection has no DB fixture.

FG remains incomplete: durable write-operation business orchestration,
capability/settings, remaining authorization audit, populated-workspace capacity
and server lock-wait evidence are not supplied by these passes. The feature
remains disabled and lifecycle/history/closure/client gates remain closed.

A subsequent investigation resolved the retained source-context cleanup failure
as a statistics-dependent multirow claim in the existing SQL. See
[s2-source-cleanup-verification.md](s2-source-cleanup-verification.md) for the
reproduction, MATERIALIZED fix and final48-entry repeated regression.
