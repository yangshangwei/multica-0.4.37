# Research: Remaining CLI/Webhook assertion scope

- Query: Can the two remaining original Go assertion skips be diagnosed meaningfully within the authorized fixed-candidate test campaign?
- Scope: internal; pre-execution research only
- Date: 2026-11-02

## Findings

**A new owned Webhook lookup-error diagnostic is feasible. CLI unknown-key preservation is explicitly unimplemented and is not a passing contract for this candidate. Neither original skip is changed or reclassified.**

### CLI: retain the documented limitation

`server/internal/cli/config_test.go:255–265` says unknown fields are dropped today and preserves the test as a future capability marker. `config.go:162–167` explicitly repeats that limitation. `LoadCLIConfigForProfile` at305–321 unmarshals into the typed struct; `SaveCLIConfigForProfile` at361–389 marshals that struct to an atomically renamed file. There is no raw unknown-key buffer/merge implementation. Top-level unknown properties, unknown backend objects and unknown fields inside a known override cannot be expected to survive. Arbitrary keys inside a declared map such as `ProfileCommandOverrides` are map contents, a separate supported case.

The existing old-file/nil-backend, known OpenClaw overrides, override-map and daemon-knob tests already cover supported compatibility at config_test.go:16,56,88,174,307. Do not rerun their full matrices or turn the future marker into a must-pass assertion. The coordinator explicitly narrowed this batch to preserving the CLI caveat without another loss probe.

### Webhook: actual owner and injection seam

The current public handler at `autopilot_webhook.go:347–382` reads the nonempty token from the path. Real absolute-IP and bad-credential-debt checks precede the query. `GetWebhookTriggerByToken` returning `pgx.ErrNoRows` produces404`{"error":"webhook not found"}` and charges bad-credential debt. Any other query/Scan error produces500`{"error":"internal error"}` before the body is read or a delivery is written. This is a current product contract, motivated by providers retrying500 instead of silently dropping404.

The old marker at `autopilot_webhook_handler_test.go:954–971` overlooks a usable existing seam: `db.New(DBTX)` at generated/db.go:14–27. The generated token method at autopilot.sql.go:1120–1181 calls the supplied DBTX.QueryRow with exact canonical SQL/token and scans20typed fields. Existing test wrappers in daemon_comment_delivery_test.go:114–151/484–485 use this same seam. A private wrapper can forward unchanged SQL/arguments to an independently owned pgxpool and record actual QueryRow/Scan errors; no Queries interface/product refactor is necessary.

### Recommended concrete permit

Use one new top-level identity `TestTaskPrivateWebhookTokenLookupErrorDiagnostic`, in a new task-private overlay whose virtual target is `server/internal/handler/zz_task_private_webhook_lookup_test.go` in the pinned Go checkout. The virtual path must be absent before and after. Preserve both original skip bodies and the entire originalGo inventory. Suggested new evidence run is `evidence/next-webhook-lookup-1`, with coordinator-allocated `multica_func7af9_next_webhook_lookup_1`.

| New subcase | Real condition | Expected witness |
| --- | --- | --- |
| server_lookup_error_returns500 | Separate owned pool with connection-local search_path=pg_catalog; no DDL or canonical SQL rewrite | HTTP500, 1 lookup(s), body reads0, no Exec/Query/SendBatch; pgconn.PgError SQLSTATE42P01, not pgx.ErrNoRows |
| closed_owned_pool_returns500 | Close only a separately constructed and identity-verified owned pgxpool before canonical lookup | HTTP500, 1 lookup(s), body reads0, no Exec/Query/SendBatch; Actual closed-pool acquisition error, nonnil and not pgx.ErrNoRows; no claim of server-side SQL execution |
| real_missing_token_returns404 | Normal separate healthy pool; a fresh synthetic valid-shaped47-byte token with no row | HTTP404, 1 lookup(s), body reads0, no Exec/Query/SendBatch; errors.Is(err,pgx.ErrNoRows) |
| absolute_limit_preempts_lookup | Precharge a real memory absolute-IP limiter with Limit1; error-pool DBTX remains ready | HTTP429, 0 lookup(s), body reads0, no Exec/Query/SendBatch; positive integer |

For the server-error case, create only an independent Config() copy with MaxConns1/MinConns0 and connection-local `search_path=pg_catalog`. The canonical query is unqualified; PostgreSQL therefore returns42P01 without schema/DDL changes. `project_health_test.go:365–367` demonstrates an existing connection-local search-path pattern, but do **not** copy its CREATE/DROP SCHEMA/VIEW setup. Record the actual `*pgconn.PgError.Code`; do not substitute a synthetic error for this case.

The closed-pool case closes only a separately constructed and identity-verified task pool before the lookup. Its generated QueryRow/Scan returns an actual pool-acquisition error. State clearly that this case does not execute SQL on the server. A healthy separate pool provides a real ErrNoRows control and the bad-credential debit distinction. Finally precharge a real memory absolute limiter with Limit1 to prove429preempts the erroring lookup, with positive Retry-After and zero DB/body calls; do not disable the safety limiters or change shared testHandler limiters.

Create a new minimal `Handler{Queries: db.New(spy), cfg: Config{}, WebhookIPRateLimiter: ownMemory, WebhookAbsoluteIPRateLimiter: ownMemory}` rather than launching a service or mutating globals. Metrics nil is supported; the429writer at autopilot_webhook.go:931–944 only records the business metric and writes its response; business_events.go:402–407 returns immediately for nil metrics. Bind a fresh synthetic47-character path token and a documentation-range RemoteAddr. Drive it with `testutil.JSONRequest`, `WithURLParams` and `Call(t,h.HandleAutopilotWebhook,req).Want(status).JSON(&out)` (http.go:22,61,93,103,128). Use a counting body reader; error/control paths must read it zero times.

The DBTX witness must count QueryRow and delegate its returned Row.Scan with all destinations unchanged. Assert the captured SQL named GetWebhookTriggerByToken and exact single `pgtype.Text` token; capture fullSQLhash and actual error type/SQLSTATE. Exec/Query/SendBatch counts are zero; no later GetAutopilot/delivery work may occur. Standard error JSON contains only its error field and no token/row IDs. Compare healthy-pool before/after fingerprints/counts for webhook_delivery/autopilot_run/agent_task_queue, and prove healthy testPool.Ping still works after faults. No domain fixture is needed for a synthetic missing-token lookup.

### Environment, command and cleanup

Handler TestMain at handler_test.go:44–102 always connects to DATABASE_URL and creates/cleans shared fixtures, even when the filter selects this one private test. MissingURL exits2; unreachableDB can exit0 without tests. The coordinator must verify a new explicit task-owned empty migrated DB, selected database identity and aggregate connection headroom first; never use the default, a browser DB, an old failed DB or an empty-filter compile probe.

Compile with the repository Agent CLI guard, `go -C server test -race -p 2 -overlay <absolute overlay> -c -o <private binary> ./internal/handler`. Execute that guarded binary with Go JSON capture, exact test filter, count1/parallel1/timeout5m. If all four subcases are chosen, require actual1top+4sub terminal events with no skip; exit0 alone is insufficient. These results are diagnostic identities, not additions/replacements to10376PASS/1FAIL/6SKIP.

Use privateHOME and explicit existingGo cache/module paths. Do not export task-config/workspaces-root or pool-default overrides globally into the test process. Own pools use Config() copies, MaxConns1/MinConns0 and bounded connection/query contexts. No agentintegration, real installedCLI discovery, Provider, router socket/API/app or remote/paid operation. Close every owned pool; leave sharedtestPool closing to TestMain, verify it remains healthy before cleanup, then let the coordinator prove selectedDBconnections0 and task process/port/mount/protocol cleanup. Retain DB and raw failures; no product repair or rerun-to-green.

## Files Found

- `server/internal/cli/config.go` — Typed CLI configuration and load/save implementation; explicit unknown-key limitation.
- `server/internal/cli/config_test.go` — Original forward-compat skip and existing canonical known-field round trips.
- `server/internal/handler/autopilot_webhook.go` — Public bearer-token ingress, rate limits, ErrNoRows/other-error split.
- `server/internal/handler/autopilot_webhook_handler_test.go` — Original skipped500marker and existing rate-limit/control journeys.
- `server/pkg/db/generated/db.go` — Existing DBTX injection seam; no product interface change needed.
- `server/pkg/db/generated/autopilot.sql.go` — Canonical token query and generated20-column Scan.
- `server/internal/handler/handler_test.go` — Mandatory database-backed TestMain and shared fixture/pool owner.
- `server/internal/handler/handler.go` — Handler fields and standard error response writer.
- `server/internal/handler/webhook_rate_limiter.go` — Real in-memory Allow/Check behavior and IP factories.
- `server/internal/testutil/http.go` — Existing request/URL-param/Call/Want/JSON helpers.
- `server/internal/testutil/db.go` — Owned fixture/count helpers.
- `server/internal/handler/project_health_test.go` — Prior independent-pool/session search_path fault pattern (do not copy its DDL).
- `server/internal/handler/daemon_comment_delivery_test.go` — Existing DBTX counting/failure-wrapper pattern.
- `server/go.mod` — Repo Go1.26.6 and pgx/v5v5.9.2 dependency identity.
- `.trellis/spec/server/database-test-isolation.md` — Explicit database, TestMain, guard, source/frozen and cleanup contract.
- `.trellis/spec/server/security-boundaries.md` — Automation principal/dispatch authority remains separate.

- `server/internal/metrics/business_events.go:402–407` — nil-safe webhook rate-limit metric; no external analytics action on this429 path.

## Related Specs and References

- RootCLAUDE testing rules: canonical package, existing testutil fixtures/calls, AgentCLIguard, no implicit real accounts.
- `.trellis/spec/server/database-test-isolation.md`: TestMain is not compile-only, explicit ownedDB, budget/source/frozen identity and cleanup.
- `.trellis/spec/server/security-boundaries.md:77–124`: automation authorization/publisher/dispatch principal contracts remain outside this early query-error diagnostic.
- Repository go.mod declares Go1.26.6 and pgx/v5v5.9.2; generateddb.go says sqlcv1.31.1. Standard API references: https://pkg.go.dev/encoding/json and https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool. No external/network research was needed.

## Caveats / Not Found

- This is researched feasibility and a proposed permit, not executed evidence. Researcher ran no test/build/DB/app/account/remote operation, performed no git operation, and wrote only the two research files.
- Candidate identity comes from supplied fixed task context; implementation/coordinator must freshly verify actual source/frozen identity before execution.
- Original CLI and Webhook skip identities remain unchanged; original2171ms>2000msP95FAIL and all platform/trust/auth/Provider gates remain open.
- A lookup-error proof does not establish valid-token signature/delivery/worker/Provider acceptance. The closed-pool and actualserverSQLSTATE cases deliberately disclose their different failure boundaries.
- Two read-only lookup batches referenced guessed absent source filenames; their tool setup rejections were corrected after rg file/symbol lookup. Exact path-existence/recovery observations are in theJSONcompanion. No429 or empty-message observation occurred.

Machine-readable permit and current inspected sourceSHA-256: `research/skipped-assertion-scope-1.json`.
