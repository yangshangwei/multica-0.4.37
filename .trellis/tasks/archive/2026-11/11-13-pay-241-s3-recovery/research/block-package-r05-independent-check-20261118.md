# PAY-241 S3 — independent R05 blocking-package check, 2026-11-18

**Verdict: PASS for the isolated package increment. TC-23 / R07 remains PARTIAL.** The candidate fixes BK-R03-02 on the three human history reads. The fresh normal and race runs agree by test name, not just by count. Neither those passes nor the API build explain the first valid-read 503. The original Web and Desktop runs both failed before submitting a block; neither proves an end-to-end unblock.

Evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`. Below, `P` = `verification/block-package-continuation-r05`, `S` = `source-block-work-r05`, and `R` = `runtime/block-information-r05`. This is a review of the original files and receipts, not a new execution.

## Findings (fixed)

**BK-R03-02 — verified in the candidate; no reviewer code change.** `S/server/internal/handler/collaboration_information.go:144` prepares a private read, sets one five-second context, parses the issue ID and calls `ResolveIssueAccess` before inspecting the query. `S/server/cmd/server/router.go:1705` registers the following GETs in the authenticated group, separately from the handoff and return POSTs:

| GET under `/api/collaboration/issues/{id}/handoffs` | Registered handler | Authorized reader |
| --- | --- | --- |
| Collection (no suffix) | `ListCollaborationInformationHistory` | Parses the raw query once; valid `limit=1` and the returned cursor reach the exact older handoff. |
| `/{handoffID}` | `GetCollaborationInformationHandoff` | Rejects a parse error **or any parsed field**; an unparameterized GET returns the requested ID. |
| `/{handoffID}/superseded-requests` | `ListCollaborationInformationRepairAffected` | Validates the handoff ID, then uses the collection parser. A valid limit on a non-repair is `404 not_found`, not an empty success. |

Previously, `r.URL.Query()` silently dropped malformed pairs. The two lists now reject any `url.ParseQuery(r.URL.RawQuery)` error, including a malformed pair alongside a valid one; they do not use the partial values. Only single `limit` and `cursor` keys are allowed. Limit defaults to 50 and must round-trip as canonical decimal 1–50; the decoded cursor string is capped at 2,048 bytes. Empty cursor retains first-page semantics. The authorized malformed cases return `400 invalid_information_query`; an unrelated human gets a content-free `404 not_found` on all three, despite malformed input. The test also retains the collection's no-store, legacy-unlinked/null-event and machine-denial controls. It invokes real handlers with injected Chi path params and a principal; route registration was checked in source, not inferred from a live request. No creation-options, writer, SQL or cursor-signing change is covered by this fix.

**Source identity.** I rehashed every file in `S`: all 5,077 paths, modes and digests match the fresh handler manifest. Comparing the six `P/*/source-before.json` records (including the nested serial run) gives the same path/mode set throughout. Against the service R04 snapshot, RED changes only the test; from RED to the first normal, fresh normal and race, only the handler changes. Reversing only the query-parser edits in memory reproduces the original production digest.

| File under `server/internal/handler/` | Original R04 SHA-256 | R05 SHA-256 |
| --- | --- | --- |
| `collaboration_information.go` | `276ec2aa84912df8d71457c75e9023fe6ee741f78db662858f2d86304d519ef1` (also RED) | `71ae10ad9cf06e05ba608a5639e7a97e34bb322e2b6eaeae06f9aad6f700fe5f` |
| `collaboration_information_test.go` | `0eb00a049f57537171617bb29204c3feec4ae8660b923d590af6dceb202b5d14` | `4524f6695bd24f902aca3aa85a0f1114b2e8405638352d7e5219bfe087dd845b` (RED and all post-fix runs) |

SHA-256 of the retained `source-before.json` inventories: service pair `f782c487e847ff3c3572eb88531a7f345cf213a0eb6c8803bc8c94897a9a762f`; handler RED `614a13dff60b2e194e343da267b53f3e8fcbbd24dc1c19830c36b0c0a7335842`; all three post-fix handler runs `02069a51d522f3b036279dee337b18c684ab6a0fe46eb57312922ffeeb4d022a`.

## Verification

I recounted `stdout.jsonl` for each profile and matched the raw SHA-256, package terminal, `result.json`, command, source manifest and allocated configuration. Each started name had one run and one terminal; no skips. These are filtered `go test -json -count=1` profiles, not the whole packages. Counts include top-level tests.

| Receipt directory under `P` | Tops | Named pass / fail | Exit | Raw SHA-256 prefix |
| --- | ---: | ---: | ---: | --- |
| `handler_red` | 3 | 9 / 6 | 1 | `54a124a002ed2582` |
| `handler_normal` (first post-fix run) | 3 | 2 / 1 | 1 | `520927d904959de9` |
| `serial-normal/handler_normal` (fresh) | 3 | 15 / 0 | 0 | `28893a845c8958fc` |
| `handler_race` (fresh, `-race`) | 3 | 15 / 0 | 0 | `71ba8e716f794989` |
| `service_normal` | 20 | 74 / 0 | 0 | `0ee179185095bcb2` |
| `service_race` (`-race`) | 20 | 74 / 0 | 0 | `3e96a871b7ca37e0` |

- **Real RED:** five wrong HTTP outcomes, plus their failing parent. Collection returned 200 on malformed-only and mixed input; exact detail returned 200 on malformed-only (mixed already returned 400); the affected list reached its non-repair 404 on both instead of returning 400. This was neither a setup nor a compilation failure.
- **Fresh handler pair:** the serial normal used `bc06_handler_normal`, not the failed `bc05_handler_normal`; `bc05_handler_race` ran after serial normal completed. Same source, filter and 15 names: metadata wire, closed repair basis, history wire, plus 12 history children (three valid controls and malformed-only, mixed, authority-first on each route). The names match RED's inventory exactly.
- **Service pair:** both commands used `source-block-work-r04`, **not** an R05 service rerun. All service bytes match R05. Each profile has the same 43 earlier blocking names plus 31 UTC/metadata names, 20 tops and 54 children. The four passing profiles thus contribute 178 named executions, not 178 business cases.
- **Static checks:** `gofmt -l` on both R05 handler files returned no paths. The retained `go vet ./internal/handler ./internal/service` and `go build ./cmd/server` receipts both exit 0. I rehashed `P/candidate-build/server`: `fc22518dc14d0a16e54d54bccfd04423f3d206064b695caeda71d9c247e6a898`, the candidate binary named by the original API launch. No frontend typecheck or new test run is claimed here.

**Gate and connections (from snapshots, not a live DB audit).** All six commands used distinct allocated DBs. `database-before.json`, `database-after-test.json` and `database-final.json` record zero **other connections to the subject DB** at all three points, not a vacant cluster. The 855-entry ledger and 663-index inventory are unchanged. Of 211 table counts, handler runs differ from baseline only by `collaboration_config_invalidation` +4; service runs only by `organization` +1, `collaboration_config_invalidation` +12 and `collaboration_knowledge_invalidation` +112. All four handler gates stayed at epoch 2. Both service tests left creation disabled at epoch 2; the guarded restoration returned it to true at epoch 3. Execution and host execution stayed off; no freeze was active. This is not a claim that all rows were byte-identical.

## Findings (not fixed)

**First valid-read 503 — unresolved.** The first post-fix normal stopped at `collaboration_information_test.go:66`: authorized `GET …/handoffs?limit=1` expected 200, got `503 collaboration_unavailable`. The other two tops passed; **none** of the 12 history children ran. The fresh pass used the same handler, test and arguments; the environment differences were confined to the subject DB and private scratch paths. It does not identify the cause of the 503 or justify erasing it. This is separate from the malformed-query RED and from the client stop.

**Original R05 Web/Desktop — 0/2 tops passed; 2/12 subcases started and failed, 10/12 not run.** `R/playwright-report.json`, `R/actual-run-01/playwright.out`, and both `test-results/*/*-information-recovery{,-A}.json` inventories agree: one worker, zero retries; A failed on each client, B/C/E/F/G never started on either. The original spec SHA-256 is `a059f1d499f65cb6a1d71cbd97938a399a630d8d8006226aa9b3d859ea293cbb`; the scenario fixture is `a4af851672a77b785ad47ec8592cb04035575e7aa0fca08a75f4ca0a95e43b28` in both setup and execution. The four attached JSON copies match their originals.

Both clients reached the managed form, observed the invalid unzoned ETA and disabled Confirm, then stopped at `harness/e2e/fixtures/collaboration-information-recovery.ts:270`. The empty-write assertion spanned recipient selection. The recorder classified lookup POSTs as writes: **search, resolve, search** on each acting client, all without an idempotency key; no handoff POST was observed in that window. The negative checkpoint was *after* the failed assertion. Setup did write data; the retained end snapshot shows `issue_handoff`, `collaboration_stage_submission`, `collaboration_run` and `collaboration_command` still zero. This distinguishes an observer assertion from evidence that an invalid block was persisted. The original runtime retained gate epoch 2 with original flags and zero other subject connections; its stop receipt lists no remaining API/Web/renderer listeners, and both native fixture processes exited 0. No installed-Main or provider execution was claimed.

**TC-23 boundary.** At the service/handler layer there is evidence for the managed reason, responsible recipient, unblock condition and zoned UTC-microsecond ETA; legacy omission and final-clock refusal; exact nested return and standalone repair to a known handler/status; immutable original receipts, supersession links/counts, pending submission protection and no implicit B2/dispatch. History draws the actual actor and time from the handoff row and reports the restored status from recorded facts, not from the ETA. The filtered tests check representative identities, statuses and exact affected paging; they do not establish that either client displayed the actual unblock person, time and restored status together.

Neither client completed A's block, historical repair, original-receipt recovery or later explicit request. Recipient removal (B), legacy policy activation (C), requester removal and protected history (E), nested returns (F), and repair preserving a genuine pending submission (G) are unexecuted scenarios in this round. **No TC-23 PASS, full S3 acceptance or integration decision follows.** No DB, service or Git operation was performed by this review; only this report was written. Any fixture, runtime or integration decision remains with the coordinator.
