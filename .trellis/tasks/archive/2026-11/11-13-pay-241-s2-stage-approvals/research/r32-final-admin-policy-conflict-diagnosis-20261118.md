# Research: R32 final administrative policy conflict

- Query: What can the original R31 final M1-policy PUT409 establish, and what is the smallest next diagnostic slice?
- Scope: internal; frozen source and retained original runtime evidence only.
- Date: 2026-11-18

## Findings

Recommend the two-file, private, passive policy observer below. The original cause remains unproven; no product repair is justified by the available evidence.

### Source and evidence locations

Aliases used in citations:

- `R31SRC`: `/Volumes/artisan/.multica-acceptance/pay241-r31-idle-claim-20261118-bz7ym_r1/source-green-r01` — accepted frozen 5,055-file source, count supplied by coordinator.
- `R31RT`: the same R31 directory's `runtime/` — retained original run and logs.
- `R24FE`: `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/frontend-base-r01` — original frontend fixture named in the actual failure stack.
- `R32SRC`: `/Volumes/artisan/.multica-acceptance/pay241-r32-policy-read-20261118-24y0i9dv/source-red-r01` — prepared fresh diagnostic source. The four product files and two observer files below were byte-compared with R31; all six match. This bounded comparison does not re-audit the full manifest.

| File found | Purpose |
| --- | --- |
| `R31SRC/server/internal/handler/delivery_policy.go` | Human policy GET/PUT, project fence, CAS, append and commit. |
| `R31SRC/server/internal/handler/agent_artifact.go` | Existing transport error mapping and artifact transaction/current-authority adapters. |
| `R31SRC/server/internal/service/delivery_policy.go` | Borrowed-transaction schema/current-policy contract; absent policy is revision0/off. |
| `R31SRC/server/pkg/db/queries/delivery_policy.sql` | Exact exclusive NOWAIT project query and append-only policy queries. |
| `R31SRC/server/cmd/server/pay241_read_diagnostics_test.go` | Existing passive HTTP/query observer, safe correlation and capture limits. |
| `R31SRC/server/cmd/server/pay241_read_diagnostics_scope_test.go` | Existing six scope leaves and three response-forwarding leaves. |
| `R24FE/e2e/fixtures/collaboration-stage-execution.ts` | GET revision followed by policy PUT; assertion occurs before returning its fact. |
| `R24FE/e2e/collaboration-stage-execution.spec.ts` | Completed original M1 journey followed by policy-off prerequisite for R38. |
| `R31RT/four-scenarios-r01/result.json` | Original four-case outcome and exact failure stack. |
| `R31RT/four-scenarios-r01/test-results/collaboration-stage-execut-365ef-nistrative-or-project-exits-chromium/stage-execution-evidence.json` | Original E04 completion/recovery facts; no final policy409 fact. |
| `R31RT/api-bridge.stderr.log` and `read-diagnostics.jsonl` | Actual policy GET200/PUT409 request logs and earlier observer exhaustion. |
| `R31RT/original-policy-pg-window-r01/result.json` | Retained narrow PostgreSQL window; strong candidate with attribution limits. |
| R32 `evidence/policy-diagnostic-boundary-draft-r01.json` | Coordinator's proposed two-file boundary, SHA256 `578faca787c0e90352013fb154721a611c99b35dbc4041d9b74a11db112649f4`. |

### Original outcome and missing fact

The original run is **3PASS / 1FAIL / 0NOT_RUN, retry0**. E04 retained the Web-issued authorization, Desktop completion using the original key, Desktop consumed GET200, and Web original-operation GET200 plus consumed GET200. Its recorded completion step at `2026-11-17T22:21:02.287Z` reports final status `done`, one actual final API write, one interrupted browser attempt and no storage copying. Original assertions573–583 were reached (`R24FE/e2e/collaboration-stage-execution.spec.ts:573`). Do not replay that successful original write.

The next call at spec587 requests M1 `off` before entering the independent R38 account-administration branch. `setStageM1Policy` reads the revision, sends PUT, then asserts200 at fixture356 **before returning** the fact (`R24FE/e2e/fixtures/collaboration-stage-execution.ts:349`). The caller's `j.retain` therefore never received the failed PUT fact. R38 account disable/takeover/restore was not reached.

API log16925 records policy GET200; log16950 records policy PUT409 for the same project `675a9750-7bb9-4aa9-bf98-08671e1016e0` and verified actor. They have distinct request IDs and no response body or CAS value. The existing observer accepts only collaboration root/R4/M1-candidate GETs, excluding both policy requests (`pay241_read_diagnostics_test.go:103`). It additionally exhausted4,096 event positions, writing its `capture_limit` marker at JSONL4097, `2026-11-17T22:20:58.15722Z`, before the final policy request. These are two independent capture gaps.

### Exact conflict branches

| Branch | Frozen source and result |
| --- | --- |
| Current authority | `delivery_policy.go:121` takes the artifact workspace/current-user/current-credential fences. PUT requires current owner/admin at131–133; refusal is403 `delivery_policy_management_denied`. |
| Project contention | PUT calls `LockDeliveryPolicyProject` at135. SQL31–32 is `SELECT ... WHERE id=$1 AND workspace_id=$2 FOR UPDATE NOWAIT`. A failure returns before reading/comparing the current policy. `agent_artifact.go:54` maps native55P03 or40P01 to409 `artifact_authority_busy`. That mapper also applies to earlier authority fences or commit errors; the code alone does not locate a holder. |
| Stale CAS | After the project fence and current-policy read, `delivery_policy.go:147` compares `out.Revision` with `req.ExpectedRevision`; mismatch is explicit409 `delivery_policy_revision_conflict`. The fixture's GET and PUT are separate transactions, so a GET200 does not prove its revision remains current. |
| Successful append | Only after CAS, SQL37–39 appends revision+1; handler159 rereads it,164 rechecks credential expiry, and109 commits before returning200. `off` bypasses only the enforce-activation schema branch, not the original schema, authority, exclusive fence or CAS. |

The original PG window contains one project-row NOWAIT error at `2026-11-17T22:21:02.745Z`, PID53791, query `LockDeliveryPolicyProject`. This closely matches the actual policy409 timing and is a **strong candidate** for the busy branch. The window has no database identity, HTTP request ID, project/workspace parameters, explicit SQLSTATE or holder transaction. Its own `exactHTTPRequestAttributionClaimed` is false. It cannot uniquely attribute that error to this PUT or establish which reader/writer held the row. Do not infer that any particular R4, policy GET or old R31 read fix caused it.

### Precise next slice and offline coverage

Only the fresh private `R32SRC` copies of these two test files should change:

1. `server/cmd/server/pay241_read_diagnostics_test.go`: replace only `pay241DiagnosticPath`'s capture predicate. Admit GET and PUT when `strings.Split(path,"/")` has exactly five segments: leading empty, `api`, `projects`, canonical nonzero UUID via existing `stageControlUUID`, `delivery-policy`. Do not hardcode the old project UUID. Exclude every old root/R4/M1 route in this new numbered slice to preserve capture capacity. Leave all sink, tracer, Tee and request-forwarding code unchanged.
2. `server/cmd/server/pay241_read_diagnostics_scope_test.go`: adapt the existing scope/fidelity tests and helper to both methods and one canonical project path. Preserve **2top / 9leaf / 11named**, with new policy names: `TestPAY241PolicyDiagnosticExactScope` has six leaves (`policy_get`, `policy_put`, `other_method`, `invalid_identity`, `other_route`, `stage_routes_excluded`); `TestPAY241PolicyDiagnosticResponsePreservation` has three (`success_passthrough`, `auth_refusal_passthrough`, `policy_conflict_passthrough`). Freeze the actual authored inventory before execution.

The six scope leaves must cover exact GET/PUT positives; rejected other/lowercase methods; zero/malformed/uppercase/braced/encoded/extra UUID forms; wrong prefix, missing/extra segments, slash suffixes and query text in the path; and explicit exclusions for old root/R4/M1, stage-policy and unrelated project routes. Query strings remain outside `r.URL.Path` and must pass through unchanged.

The three forwarding leaves can iterate GET/PUT without adding named subtests. Exercise synthetic200, pre-query/auth refusal and409 policy-conflict payloads, including busy/CAS codes as offline data. Retain direct/nil/excluded/admitted forwarding, original status/headers/body/chunk-write results, request body, request-ID/cancellation/verified principal/session context, one callback per invocation, 1MiB prefix capture with full-byte digest and completeness, and the last record/limit marker/ignored tail at4,096. A synthetic oversized policy-conflict body tests observer fidelity; it is not a native policy response. HTTP-only callbacks must still leave query/PID/SQLSTATE and query-derived identities absent rather than fabricate them.

Keep the existing dedicated harness enablement, exclusive0600 sink in a0700 parent, 4,096-event/1MiB limits, safe request/query correlation, actual backend PID, SQL hash/name, SQLSTATE/error type and frames unchanged (`pay241_read_diagnostics_test.go:57`,87,129,162,216). No request-body collection, SQL arguments, bearer values, pool changes, retries, permission changes or timeout changes are needed.

After independent source/offline acceptance and coordinator-owned matching builds, run the **original four scenarios once** in a new numbered binding, using the original assertions and5/20/480-second deadlines, workers1/retries0/maxFailures1. Capture the actual policy GET revision/body and PUT response/code, correlated request trace/PID/native query error where present. GET success plus PUT busy/CAS separates the branches; missing/truncated/native-unattributed capture must remain unknown. Even a correlated busy response does not itself identify the holder or justify weakening a legitimate writer fence. A demonstrated product defect then requires its own same-layer regression and bounded repair.

### Related specs and references

- `.trellis/workflow.md` §1.2 requires persisted, task-owned research; this dispatched role does not run main-session implementation or verification.
- `prd.md` A08/A09/A11/A12 and `design.md` project-off/CAS, lock/recovery and final-consumer contracts preserve current authority, historical completion, explicit policy rollback and independent R38.
- `.trellis/spec/server/delivery-policy-evaluations.md` §§3–4 requires project-row serialization, no retry and409 contention/CAS refusal; `service/delivery_policy.go:44` preserves transaction-local current-policy reads.
- `.trellis/spec/server/collaboration-stage-approvals.md` keeps real writers/current-authority fences and original runtime evidence distinct from focused test acceptance.
- Local primary dependencies: Go1.26.6, pgx/v5 v5.9.2, Chi/v5 v5.3.0 (`R31SRC/server/go.mod:3`,11,18). No external lookup or new dependency is needed for this existing contract.

## Caveats / Not Found

- No `research.jsonl` exists. The role explicitly forbids loading `implement.jsonl`/`check.jsonl`; that coordinator request was reported upward, and context used `prd.md` → `design.md` → `implement.md`, relevant specs and narrow source instead.
- Original policy409 response/request payload and exact PG/HTTP/holder attribution remain missing. No product defect or repaired outcome is claimed.
- No Go, database, API, browser, application/process-control or test execution was performed by this researcher; only file inspection, bounded hashing and these two new research outputs. Coordinator's fresh router-profile allocation is preparation, not executed evidence.
- The coordinator reports stopped API/bridge, zero connections, restored gate74,64settled/48complete and preservation of338old/381artifact paths; these were not re-audited here. Accepted R31 product controls/builds remain supplied prior evidence.
- Parent24 remains0PASS/9PARTIAL/15NOT_RUN and fullS2 remains false. R31's accepted original completion and policy-off failure do not establish the pending R38 branch, fullS2, realS4 or finalS5 acceptance.
