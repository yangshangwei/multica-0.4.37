# S2 server increment 2 — coordinator brief

Owner: coordinator session -3a (2026-11-14). The user chose to implement increments 2 and 3 before running A01–A12 (implement.md, "2026-11-14 接管与推进决定"). This file is the brief for each server sub-increment, so a successor can continue if a session or lane dies.

## Order and ownership

One server owner at a time. Sub-increments run in sequence because they share the stage tables, `collaboration_stage*.go`, the workflow mutation, and the delivery completion files. Each one stops at its boundary, writes `research/server-increment-2X-results.{md,json}` (same structure as `server-increment-1-results.md`), and waits for coordinator review.

| Sub-increment | Scope (wire-contract sections) | Unlocks |
| --- | --- | --- |
| 2a | Signer generation and expiry (8.1). Code candidate A6 (8.2). R4 (8.6) with `observed.pull_request`, `observed.ci`, `decisions`, `signable`. GitHub PR/CI evidence and the server `ci_digest`. A7 `code_review` approve/return (8.3). A8 revoke (8.4). A9 `development → code_review → qa` and rework `→ development` (8.5). Reopen resets stage (10.2 table). | A03, A04, part of A08 |
| 2b | T1–T4 typed runtime test report (8.7, `test-report-contract.md` §2–8): `test_subject` bound at create-only typed Agent input, frozen on first claim. One immutable report per run/attempt as `run_event(kind=test_report)`. CLI `collaboration test-report subject/submit/receipt`, plus the builtin skill docs. A7 `qa` approve/return. A9 `qa → release`. R4 `test_reports`. | A05 |
| 2c | Internal execution-fact boundary for deployment/rollback (design F05 table; no public write route, no simulation bypass a production caller could choose). A7 `release`/`rollback`. A9 `release → verification`. R4 `execution_facts`. | A06, A07 |
| 2d | 10.2 gates on submit, complete, R38, project move/clear, and project delete. The private stage guard carried by `PrepareDeliveryMutationInTx` on both new-completion branches, consumed by every real writer before commit, rechecked at `clock_timestamp()`, in all M1 modes off/observe/enforce. Replay returns history. Activity safe-projection allowlist for the stage event kinds (section 9). Readiness extended to the new objects. Flip `stagePolicyActivationAvailable` last, after every writer consumes the guard. | A09, the enabled policy needed by A01/A02/A11 |

## Rules for every sub-increment

- The wire contract (`research/wire-contract.md`) is frozen, and a client lane codes against sections 8, 9 and 10.2 in parallel. Do not change any request or response shape on your own. If the code proves a shape wrong, stop and report. Clarifications go into a dated section-13 entry, and the action's state in section 1 changes from `inc2` to `inc1`-style "served" when its route is registered.
- Design obligations are binding: design.md F01 (signer generations, no bearer storage, re-adding never revives an old approval), F02 (version layering: candidate content identity vs CAS revisions; ordinary status changes do not invalidate reviewed code), F03 (final guard), F05 (evidence sources and forbidden inferences), and the entry-point table "实际写入与准入接线清单". The GitHub gate requires all of the following and never inherits the UI fallback: a non-empty, complete check set; observed at or before the DB time and no more than 30 minutes earlier; every check `status=completed` and `conclusion=success`. VCS without a repository binding is `stage_ci_unsupported`.
- Red-first per behaviour; keep the first failing log. Tests use `server/internal/testutil` fixtures (`dbfx`, `testutil.Call`). Synthetic external facts must be labelled synthetic in the fixture and never presented as real integration.
- Migrations: next free number after 1b, no FK or cascade, and every index in its own single-statement `CREATE [UNIQUE] INDEX CONCURRENTLY` file. Run `make sqlc` after SQL changes.
- Databases: s2_impl_service and s2_impl_handler for normal runs, s2_race_service and s2_race_handler for `-race`. Use them only through `/Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs`. You may migrate exactly these four with `go -C server run ./cmd/migrate up` and must record it. Logs are `s2-server-inc2X-*.log` in that logs directory, each with a directly captured `$?`.
- Known failures not from S2: `TestBatchUpdateIssues_AutopilotRunAssign`, and `TestWorkspaceDeletionManifestCoversPublicSchema` on `report_snapshot*`. Every new stage table must be classified in the deletion manifest.
- No git writes. No API, Web or daemon processes. Edit only `server/**` and S2 `research/`; the builtin skill docs are allowed in 2b only. Never revert other sessions' files.
