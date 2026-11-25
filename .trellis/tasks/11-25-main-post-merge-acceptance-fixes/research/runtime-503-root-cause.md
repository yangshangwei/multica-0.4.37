# Research: Host 503 classification and cancellation mechanism

- Query: Which actual routes failed in final03, and which evidence distinguishes cancellation from service failure?
- Scope: internal; retained raw logs, current source, no-DB deterministic controls
- Date: 2026-11-25

## Findings

The historical **320 order / 116 memory** 503 counts are verified from raw logs; their SHA256 values match original receipts. All ten launch permits were 200. Order has 315 GET failures plus two recording/seal and three ordinary daemon/tasks/claim POST failures. Memory has 116 GET failures, including two stop-intent reads. Therefore these totals are not permit-failure counts.

| Normalized route | Order | Memory |
| --- | ---: | ---: |
| GET /api/collaboration/attachments/drafts/:id | 1 | 0 |
| GET /api/collaboration/creation-options | 2 | 0 |
| GET /api/collaboration/issues/:id | 42 | 10 |
| GET /api/collaboration/issues/:id/activity | 3 | 3 |
| GET /api/collaboration/issues/:id/attachments | 2 | 1 |
| GET /api/collaboration/issues/:id/commands | 1 | 0 |
| GET /api/collaboration/issues/:id/comments | 1 | 3 |
| GET /api/collaboration/issues/:id/knowledge-candidates | 170 | 59 |
| GET /api/collaboration/issues/:id/memory | 0 | 1 |
| GET /api/collaboration/issues/:id/mention-targets | 14 | 4 |
| GET /api/collaboration/issues/:id/reactions | 1 | 0 |
| GET /api/collaboration/issues/:id/resources | 7 | 8 |
| GET /api/collaboration/issues/:id/resources/identity | 3 | 1 |
| GET /api/collaboration/issues/:id/resources/offers | 3 | 2 |
| GET /api/collaboration/issues/:id/stage | 5 | 0 |
| GET /api/collaboration/runtime/capabilities | 16 | 16 |
| GET /api/collaboration/runtime/runs/:id/stop-intent | 0 | 2 |
| GET /api/me/collaboration-notifications | 19 | 3 |
| GET /api/me/workbench | 25 | 3 |
| POST /api/collaboration/runtime/runs/:id/recording/seal | 2 | 0 |
| POST /api/daemon/tasks/claim | 3 | 0 |

Knowledge candidate GETs account for 170/59 failures, while the same routes also complete 212/40 HTTP 200 reads. Across all 503s, order median31.187ms/max452.373ms; memory median57.955ms/max1.473434s. Short status503 records do not establish long SQL, pool starvation, or a permanently missing feature. GET503 bursts often share completion milliseconds across several panels. Historical logs omit error code, request context state and client outcome.

### Proven mechanisms, deliberately limited attribution

1. `server/internal/middleware/password_auth.go:15` maps a canceled database call to503/auth_unavailable, just as it maps an actual infrastructure error. Its requests can terminate before X-User-ID is populated. Absence of that ID is not proof of cancellation, but its presence is not required for an authentication-layer503.
2. `handler/collaboration.go:93` maps an untyped error including context cancellation to503/collaboration_unavailable. Workbench, notifications, creation options, issue detail, drafts/files, resources/identity/offers, commands, mention targets, stage, comments/reactions/activity/memory all delegate failures to this or equivalent content error paths. `service/collaboration_knowledge.go:48` additionally maps canceled errors to knowledge_unavailable; `collaboration_knowledge_view.go:234` passes request context through BEGIN, snapshot SET, source/account/credential reads and COMMIT. These are mechanisms, not retrospective classification of each individual record.
3. `middleware/request_logger.go:115–176` logs the attempted writer status after the handler and assigns ERROR to all >=500. It does not establish that the client received that status, record `r.Context().Err()`, or record safe domain code.
4. `packages/core/realtime/knowledge.ts:30–37` invalidates the entire private knowledge scope on both collaboration:changed and collaboration:workbench_changed. `knowledge-cache.ts:27,39–48` immediately cancels matching in-flight requests, erases old projections, then refetches. `knowledge-queries.ts:35–38` supplies Query's AbortSignal. The server emits issue and workbench hints from the same ordered outbox (`service/collaboration_realtime.go:140–155`), and runtime progress uses that outbox (`:169`). This is a concrete source of redundant cancellation/refetch work, even with one responder per connection.
5. Runtime-capability GET and stop-intent GET are not plain snapshot SELECTs: `collaboration_runtime_preparation_capabilities.go:149` and `collaboration_runtime_capabilities.go:590` use `collaborationRuntimeMutation`; that wrapper (`collaboration_runtime.go:450`) may also end in typed collaboration_busy after bounded serialization/deadlock/lock conflicts. Recording seal shares mutation semantics. Do not classify their503s as canceled solely because some browser reads are canceled.
6. Ordinary daemon claim can503 at managed runtime validation (`handler/daemon.go:92–104`) or authentication. Its downstream task claim error otherwise uses500. It must be analyzed separately from collaboration permits.

### Executed controls

`runtime-cancellation-control_test.go`, overlaid as a virtual middleware test (virtual path never created), ran under the pinned Go1.26.11, CLI guard, p1/parallel2, with external HOME/TMP and no DATABASE_URL. **One test PASS, exit0**, 5.82s including compilation. It uses the actual passwordAuthError and RequestLogger with an actual httptest HTTP connection: client cancellation produces context.Canceled at the server and ERROR/status503 in the log even though the client receives cancellation. A second uncanceled synthetic infrastructure error produces actual503/auth_unavailable and the same log shape. The receipt and raw JSONL retain exact tool/source/argv. No authentication lookup, account, database or model is accessed.

`runtime-knowledge-cancellation-control.cjs` transpiles and executes the current three realtime/cache/lease source files with the actual installed TanStack Query; only authority/session and valid hint schemas are synthetic test seams. Initial pending read: starts1/aborts0. An accepted issue hint: starts2/aborts1. A subsequent workbench hint: starts3/aborts2. **Exit0** with exact source hashes in JSON. This proves the cancellation/refetch mechanism, not the provenance or share of historical503s.

### Smallest next experiment / repair boundary

Capture one newly bound Host run with safe completion fields: request_id, request_context (none/canceled/deadline), attempted status, allowlisted domain code, pgx query count and summed completed durations/PID. Browser observation must distinguish response503 from requestfailed/AbortError using the same request ID. Do not capture tokens, SQL arguments, raw SQL/error bodies or browser storage. Preserve actual failures and old totals.

The smallest safe product change now is **classification observability** in `middleware/request_logger.go` and its tests: add request-context state alongside existing status; preserve status, severity and error payload unchanged. Required tests cover context_canceled, context_deadline, and context_none, plus an uncanceled real503. No response-body parsing or new error schema is required. Do not alter auth or business responses, suppress genuine unavailable, or translate all503 into a benign class. If the bound run proves redundant paired-hint churn dominates, evaluate coalescing refetch dispatch while retaining synchronous invalidation, cancellation of stale authority and generation/lease fences. A time-based debounce or ignoring hints would need separate safety proof and is not recommended from these observations alone.

The initiating historical permit failure remains unknown because its original cancellation and query timeline was never recorded. The two controls close the mechanism question, not that historical attribution. There is no evidence establishing a shared initiating cause with CLI/service slowness.

## Files found

- Original `runtime/final03/{order-export-01,memory-runtime-02}/api.log`: complete access logs, receipt hashes matched.
- `runtime-503-route-audit.json`: exact route counts, request IDs, original line numbers and per-request duration; no bodies/credentials.
- `runtime-cancellation-control-receipt.json`, `.jsonl`, `.stderr`, overlay and test source: actual middleware control.
- `runtime-knowledge-cancellation-control.cjs` and `.json`: real source/Query cancellation control and bindings.
- `packages/core/realtime/knowledge.ts`, `collaboration/knowledge-cache.ts`, `knowledge-queries.ts`: invalidation, cancellation and signal propagation.
- `server/internal/middleware/request_logger.go`, `password_auth.go`: lossy historical logging distinction.
- `server/internal/handler/collaboration.go`, `server/internal/service/collaboration_knowledge*.go`, runtime mutation files: error paths above.

## Related specs / external references

- `.trellis/spec/server/collaboration-host-execution.md`: preserve source/transport/context attribution and native execution guarantees.
- `.trellis/spec/server/database-test-isolation.md`: sampling limitations, independent package resource ownership.
- No external API assumptions were needed for these source-grounded controls.

## Caveats / Not Found

No complete browser network trace or response error-code inventory was retained for those successful journeys. They passed their scenario assertions, which checked pageerror and business outcomes, not every response. An ERROR503 log may represent an attempted response to a canceled client. This report does not relabel any original failure as PASS, change security guards, or propose suppressing503 counts. No product source was edited.
