# Webhook token lookup-error diagnostic

**PASS for this new diagnostic: one top-level and four subcases passed with race enabled. Original QA-PERF-001 remains FAIL at 2,171ms > 2,000ms; required acceptance gates remain open.** Neither original Go skip was changed or reclassified.

Native Trellis implement child `/root/webhook_lookup_diagnostic` wrote only the new private overlay/runner, `evidence/next-webhook-lookup-1/` and this report/JSON companion. Child self-verification is distinct from coordinator verification and the later independent review. The complete saved hook context, root CLAUDE.md/session-prompt and adopted execution permit were read before writing code.

## Identity and actual execution

- Candidate: `7af9ab9905c98c77bb091223ebe687c639475bb4`, version `0.6.8-rc.20261102`; isolated Go checkout `/Volumes/artisan/code/2026/mc-functional-go-7af9ab990-20261102`.
- New coordinator-allocated database: `multica_func7af9_next_webhook_lookup_1`. Child preflight verified actual identity, zero other connections, empty user/workspace/issue counts and 751 migrations. Actual headroom was 55/100 immediately before execution; this is a new timepoint, not historical load attribution.
- Toolchain: `go version go1.27.0 darwin/arm64`; dependency `github.com/jackc/pgx/v5 v5.9.2`. The repository/CI Go1.26 toolchain was not exercised by this run.
- One compilation attempt and one execution, from `2026-11-02T10:08:15.675685+00:00` to `2026-11-02T10:08:18.683298+00:00`, exit0; actual terminal events are **1 top-level PASS / 4 subcase PASS / 0 FAIL / 0 SKIP**. No executed retry or prerequisite correction was needed.
- Exact new identity: `TestTaskPrivateWebhookTokenLookupErrorDiagnostic`; no P95, cached/statistics fixture, old Go case, E2E/DEB/Mac subgate, account or Provider execution occurred.

The command used the repository Agent CLI guard for both `go -C server test -c -race -p 2 -overlay <private overlay> -o <private binary> ./internal/handler` and the compiled binary through `go tool test2json`, filtering only `^TestTaskPrivateWebhookTokenLookupErrorDiagnostic$`, `-test.count=1 -test.parallel=1 -test.timeout=5m`. Private HOME/cache paths and the explicit task DATABASE_URL were parsed from the recorded private environment. Credentials were not printed or sourced. Handler TestMain performed its normal synthetic fixture creation/cleanup solely in this new database; the diagnostic added no domain fixture.

## Four actual failure/control witnesses

| Subcase | Observed request result | Exact failure/control boundary |
| --- | --- | --- |
| server_lookup_error_returns500 | HTTP500 `{"error":"internal error"}`; one canonical QueryRow and generated20-destination Scan | Independent pool's connection-local search_path=pg_catalog produced `*pgconn.PrepareError`; errors.As reached PostgreSQL PgError42P01. This is a real server prepare/parse failure, before SELECT executor work or a returned row. |
| closed_owned_pool_returns500 | HTTP500 with the same standard error; one canonical QueryRow/Scan | An identity-verified independent pool was closed before the request. Actual `closed pool` acquisition error; the request did not execute SQL on the server. Setup identity reads occurred before close. |
| real_missing_token_returns404 | HTTP404 `{"error":"webhook not found"}`; one canonical QueryRow/Scan | Healthy independent pool returned real `pgx.ErrNoRows` for a synthetic unissued47-character token. Bad-credential debt changed from allowed to exhausted. |
| absolute_limit_preempts_lookup | HTTP429 `{"error":"rate limit exceeded"}`, Retry-After60 | A fresh real memory absolute limiter was precharged at Limit1. QueryRow/Scan and body reads were all0; the bad-credential debt was not charged. |

All three request QueryRow calls forward exact current `GetWebhookTriggerByToken` SQL and the original `pgtype.Text` argument to pgx without substituting an error, query or result. Each forwards the actual generated20-destination Scan. The server-error case fails during prepare before bind/execution, so it does not prove server-side parameter binding or successful typed-row decoding. The raw witness records canonical SQL/hash, Scan destination types and synthetic-argument equality/hash; no live bearer token exists or is disclosed.

All four paths had **body reads0 and downstream Exec/multirow Query/SendBatch calls0**. These are request witness counters; TestMain and fingerprint/identity audit reads are separate. Three selected table fingerprints/counts—webhook_delivery, autopilot_run and agent_task_queue—match before/after each request. The healthy shared testPool still pinged and retained its search_path after every fault. Only independently constructed pools were closed; no global testPool or testHandler was mutated. Both500 paths preserved the bad-credential budget, while the actual404 consumed it. Standard error JSON contains only its error field.

## Preservation, verification and cleanup

The source patch SHA-256 stayed `f78404b75d10b2527e0bcef236f86fe2504901df1625a989a23e99f77db46fbe`; frozen iteration_i1_p95_test.go stayed `fc388118311b3b3dc2c7fbec91d8acbba03a0caa5451c58df8f48b442e9cb6ef`. All10watched files, including both original skip files, remained byte-identical. The new virtual overlay path was absent on disk before and after; private test/binary/overlay/environment hashes bind the exact execution.

`verification.json` recomputes Go terminal events, all13recorded command stdout/stderr hashes, canonical SQL/argument/scanner facts, all four witnesses, process-group evidence and source/frozen identities. Private Python AST, actual Go compilation/race execution, gofmt and git diff --check passed. Full product lint/typecheck/full suites were NOT_RERUN: only task-private diagnostics and evidence changed, with no product/TypeScript/dependency change.

All four owned pools closed, the recorded task database had0other connections after TestMain ended, and both compile/test process groups had no remaining process. The DB and raw evidence are retained. This child created no service/listener/application/container/mount/protocol registration and performed no account/commercial/remote operation. The coordinator owns the broader fresh whole-task cleanup. No tool429 or empty-message issue was observed in this child; the HTTP429 above is an expected product limiter result.

## Remaining limits

This is early HTTP-handler query-error/admission evidence, not full router, valid-token signature, delivery, worker or Provider acceptance. Fingerprints cover three named tables, not every database object. It supplies a separate diagnostic identity and does not replace the original `TestWebhookHandler_DBErrorOnTokenLookupReturns500` skip or alter the original10376PASS/1FAIL/6SKIP inventory. CLI unknown-field preservation remains the documented unimplemented capability; no new preservation/loss probe was executed. Original performanceFAIL and native/auth/updater/trust/Provider gates stay open.

Raw entry points: `run.json`, `diagnostic.stdout`, `canonical_sql.txt`, four `case-*.json`, `cases.json`, the copied private test/runner/analysis files and `verification.json`. Coordinator permit: `research/webhook-lookup-execution-permit-1.json`; research boundary: `research/skipped-assertion-scope-1.md/.json`. No product fix, SQL/index/DDL/dependency change, refactor, commit or publication occurred.
