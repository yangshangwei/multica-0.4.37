# Backend verification — final product regression and performance passed

As of 2026-11-09, all required default and controlled backend correctness lanes have complete passing evidence. **The final product passed all3 original performance tests and complete service/handler/iteration race suites**, so this is not final whole-task acceptance.

| Scope | Current accepted evidence | Limitation |
| --- | --- | --- |
| Default Go race | 82 packages PASS; 10,459 top-level PASS / 20 SKIP, 21,241 named PASS / 21 SKIP | Multi-run closure: original service40m timeout and inherited-umask storage failure retained |
| Static analysis | `go vet -p 2 ./...`, plus affected agentintegration/knowledgeintegration router vet PASS | Go1.27 darwin/arm64 differs from CI1.26 |
| Knowledge composite | 23 unique tests, normal+race =46 passing top-level executions | Real router/DB/daemon, controlled provider |
| Context fake framework | 14 top-level PASS | Clean committed source snapshot required; no commercial model |
| RK1/RK1K/continuation | 4 top-level /106 named PASS in serialized final3 | Original overlapping-load checkpoint failures and pending-seal failures preserved |
| Recording recovery regressions | 2 physical cases +10 classifier cases PASS; RED before repairs | One physical provider; exact retained operation/receipt replay |
| Linux conditions | 10 top-level +3 nested PASS, zero skips | Static current arm64 binaries in actual Linux container; no installed GUI/Windows/iOS coverage |
| Triage Go→TS wire | 1 actual export +7 current schema/parser cases PASS | Uses the actual exported JSON |
| I1 + Triage scale performance | Final3/3 PASS: closure1662.866ms; disableapply1937.854ms | Original2000/3000ms limits unchanged; original failures retained |
| Public GitHub skill import | Attempted, external HTTP403 rate-limit failure | No credentials borrowed; remains external-dependency FAIL, not PASS |

Product fixes batch running-task counts and pipeline canonical participation locks for large iteration operations. Eight implementation/query/generated files plus one new service regression are listed in the private product-final-hashes.json. Four additional tagged test files repair bounded receipt recovery, preserve mixed permanent errors, and establish expiry precisely between preparation and authorization. The only spec edit is .trellis/spec/server/iterations.md. Product deadlines, sample counts, authorization and failed-provider assertions remain unchanged. Formatting, sqlc idempotence and static checks pass.

Selected evidence: /Users/artisan/.multica/qa/core-20261108/backend/accepted-default-summary.json, accepted-default-inventory.csv, accepted-conditional-summary.json, performance-3/result.json and final-source-audit-v3.json. The final source audit records nine modified tracked files and four new files against the initial 4,533-file backend/guard snapshot. Final product hashes match the tested bytes. All 80 recorded backend databases are retained with zero connections; all backend runners and owned containers are stopped.

Remaining ordinary skips are explicit: Windows behavior, unavailable pg_bigm, known CLI unknown-field placeholder, webhook error placeholder and absent HTTP MCP catalog recipe. Linux and wire conditionals are closed separately. No paid-model/provider account accessed; all fake commands are test-created and ambient agent CLI guard stayed active.

The remainder is the chronological evidence trail, including original failures and superseded preparation notes.

---

# Backend verification — in progress

Current source HEAD: `7253784066d51a53432687aeada8cc208a31a943`; dirty product files are recorded in the parent task. Server/guard file hashes are retained privately in `source-before.json` (4533 files).

Private evidence: `/Users/artisan/.multica/qa/core-20261108/backend/`.

The live default runner is `run.py`, recorded exec session 31194, log `runner-2.log`, machine-readable status `result.json`. Each DB-backed package receives a newly created, migrated database; the service package runs first. All packages run serially under `go test -race -count=1 -timeout=40m -p 1 -parallel 2 -json`, with the ambient agent-CLI guard. Each package gets a Git-external private HOME/TMP. Go caches remain pinned to their existing locations. PostgreSQL preflight: max_connections 100, aggregate connections 49. Dedicated disposable Redis uses the existing local `redis:7-alpine` image and is recorded in `resources.json`; no shared Redis data is used.

Toolchain: Go 1.27.0 darwin/arm64 (different from CI Go 1.26).

Initial launcher-only failure: psql's PGDATABASE environment value did not expand the connection URI, so the local socket probe failed before database creation or test execution. Original `runner.log` is retained; runner 2 passes explicit PG connection fields. This is not a product test failure.

After the default packages and `go vet`, `supplemental.py` will run separately identified controlled Knowledge/Context/RK1/RK1K suites with their required fresh DB prefixes. No real-model opt-in or installed provider executable is used. Results are pending; this document does not claim passing acceptance.

## Default race run completed (2026-11-09)

82 packages finished: 80 package PASS, 2 package FAIL; `go vet -p 2 ./...` passed. This original full run is not green.

- `internal/service`: aggregate 40-minute Go package watchdog fired after 1,344 of 1,450 top-level tests passed (3,133 named terminals), with no assertion failures. The active test had run only 2 seconds. Original database retained and zero connections confirmed. Another unowned service suite competed during the first ~28 minutes. A fresh complete-package replay uses a 90-minute harness budget; product assertions/latency thresholds stay unchanged.
- `internal/storage`: `TestLocalStorageUploadsAreWorldReadable` expected 0644 but observed 0600 because the evidence runner's private umask 0077 was inherited by the child. No product code changed. Complete-package replay restores conventional child-only umask 0022; private evidence directories remain 0700.

Corrected/supplemental runner is live with exec session 29602, log `supplemental-runner.log`, status `supplemental/result.json`. It first replays storage and the entire service package, then executes the six controlled tagged lanes on separate fresh databases. Dedicated Redis is new for this runner.

Performance runner `performance.py` is prepared, not yet executed. File contains exactly `TestIterationI1P95` and `TestIterationI1DisableP95`; original 2000 ms and 3000 ms thresholds remain. Parent coordinates competing-load drain before launch.

## Default package closure

Complete fresh service replay: **1,450 top-level PASS / 0 FAIL / 0 SKIP**. Storage replay: **48 PASS**. Selecting complete passing evidence per package yields **82 package PASS**, **10454 top-level PASS / 20 SKIP / 0 FAIL**, with 21232 named passes including subtests. `go vet` passed. Original failures remain unchanged; this is multi-run closure, not a green original full run. Exact selected inventories: `accepted-default-summary.json` and `accepted-default-inventory.csv`.

Root HEAD advanced to `6201a58102668ce19e5f2959ea7dfb807d932a43`; `git diff 725378406..HEAD -- server scripts/go-test-with-agent-cli-guard.sh scripts/agent-cli-command-names.txt` is empty. Backend source bytes remain equivalent.

## Controlled conditional findings and repairs

- Ordinary Knowledge composite: 14/14 PASS under race and 14/14 PASS normal.
- Context fake framework: original 13 PASS / 1 FAIL because the root server module includes existing dirty documentation and untracked fixture files; source cleanliness guard refused correctly. Replayed the exact selected source in a locally committed isolated snapshot: 14/14 PASS.
- RK1 fake proof: original launcher failed to create explicit proof output parent. Corrected launcher creates it; fresh complete proof replay pending.
- RK1K Claude: K1-corrected provider finished once and termination proof was acknowledged, while recording seal remained retained after the independent one-second telemetry budget. Fixture incorrectly asserted completion after only the initial foreground call. Added a deterministic regression delaying the outcome by 1.2 seconds: RED (`context deadline exceeded`) before repair. `rk1_run_test.go` now executes a bounded recovery-only Runner from the original retained ledger only for a recoverable deadline; it has no Host or Controller. GREEN regression proves one physical provider, one claim/outcome/proof/seal, and the identical permit replay used by initial authorization and BeforeStart. The first green attempt caught an incorrect regression expectation of one permit request; original implementation intentionally replays its permit at BeforeStart, now pinned by byte equality. Full Claude/Codex/continuation replays pending.
- Host Knowledge composite: original normal 7 PASS / 2 FAIL and race 8 PASS / 1 FAIL. Five-second fixture expiry could elapse before the second preparation; the test then did not exercise expiry between prepare and authorize. The fixture now allows 30 seconds for setup, explicitly requires its exact candidate/version in the second preparation while still live, then waits for actual expiry before the unchanged refusal assertions. Successful runs also use retained-receipt recovery after telemetry deadline; negative/refusal runs remain direct. Full normal/race replays pending.
- Go→TS Triage wire contract: actual Go export PASS; all seven exported payloads parsed by current TypeScript schemas and `parseTriageResponse`, 7/7 PASS.
- Public GitHub skill import: opt-in attempted, FAIL due upstream GitHub tree HTTP 403/rate limit. No credential access or product assertion change; external dependency limitation retained.

Private source snapshot: `backend/source`; copy manifest contains 4669 selected server/script/config files including existing dirty/untracked source/assets. Snapshot commits are local only. Baseline default backend source is unchanged; the candidate test variant adds `rk1_recording_recovery_test.go` and modifies `rk1_run_test.go`, `collaboration_knowledge_host_composite_test.go` only. Linux static execution and three opt-in performance cases remain pending.

## Conditional continuation

Linux current-source static arm64 binaries: **10 top-level and 3 nested cases PASS, 0 FAIL/SKIP**. `linux4/result.json` and `linux4/named-inventory.json` bind the executed binaries and exact identities. Initial container attempts failed before tests because this Docker daemon could not bind host paths; a subsequent stopped-container copy was rejected by read-only rootfs. Final isolated containers use `docker create/cp/start`, no host mount, task-private tmpfs, copied 0755 binaries, and agent CLI guard. Containers removed after completion; no host application/profile/model access.

Host expiry repair was refined after the intermediate 30-second TTL caused the before-send hook to exceed the existing 10-second HTTP deadline. Final targeted case PASS creates a separately approved 5-second candidate immediately before second preparation, asserts exact live candidate/version in that preparation, and waits for its actual expiry before authorization. No immutable candidate row update or product timeout change. Full host normal/race matrix replay remains required.

The full RK1 fake replay retained H3a/H3b recording failures under overlapping DB matrix load. PostgreSQL original logs show checkpoint cancellation while waiting on user-row and status-catalog locks, consistent with the unchanged 1-second checkpoint budget. H3b had repeated 503 checkpoints and then a settled-proof 403, so later metadata recovery cannot recover that missing content. A fresh isolated H3b diagnostic collects task-database lock blockers before any further repair; no pass is inferred.

## Recovery regressions and serialized final tagged run

The last serial RK1 failure exposed a distinct sanitized `ErrTransport` after a lost seal response. It left a confirmed, proof-acknowledged ledger with the original seal pending. Added a second deterministic regression which commits the seal then drops its response: original fixture RED with `collaboration runtime transport failed`, fixed fixture GREEN. Recovery accepts deadline/transport errors only while the outer context is live and the exact retained run is confirmed and recoverable; a recovery-only Runner has no execution capability. Two regressions PASS, preserving one provider launch, one original run/claim/outcome/proof, identical permit replay, and byte-identical seal replay. Failed-provider cases remain in the unchanged full fake matrix.

Final serialized controlled run `tagged-final2/result.json`: **RK1 full fake proof PASS; RK1K Claude full fake proof PASS; RK1K Codex full fake proof PASS; Codex continuation controls PASS**, all zero skips. Earlier overlapping/transport failures remain in original logs. The isolated H3b diagnostic also passed with 76 PG samples and only one sampled ~78 ms lock wait; this supports the chosen serialized environment but does not reconstruct every earlier blocker.

Host normal/race final matrix is running separately. Final fixture hashes are in `fixture-final-hashes.json`. No production source or latency threshold changed.

## Controlled scope completed

Final host composite **9/9 normal + 9/9 race PASS**. Together with ordinary Knowledge **14/14 normal + 14/14 race**, all **23 unique composite tests / 46 executions** pass. Final RK1/RK1K/continuation selection has **106 named PASS**, no failures/skips. Context fake framework has **14 top-level PASS**. Recording-recovery regressions **2 PASS**. Linux **10 top-level + 3 nested PASS**, actual Go wire export **1 PASS** and TypeScript parser **7 PASS**.

Selected evidence is reconciled in `accepted-conditional-summary.json`; original failed runs are not overwritten or added to passing counts. All backend execution processes are terminal. Performance remains held for the parent-controlled idle measurement window.

## Independent review correction (current test variant)

Independent review found that matching any deadline/transport member of `errors.Join` could erase an unrelated execution/persistence error when recovery succeeded. Added a shared tagged test-only classifier that recursively visits both single and multiple unwrap forms and admits recovery only when **every terminal leaf** is exactly deadline-exceeded or transport failure. Nil, cancellation, protocol errors and mixed permanent errors retain the original failure.

The old predicate failed three deterministic mixed-error cases (provider+transport, protocol+deadline, nested permanent+deadline). The final classifier passed all10 table cases and both actual delayed/lost-seal regressions: **3 top-level /13 named PASS** in `recovery-final/result.json`. Independent reviewer found no remaining blocking source issue. New test variant re-runs the complete tagged matrix (`tagged-final3`, running) then host normal/race (`host-final2`, queued); prior green results are retained for their previous exact fixture bytes.

## Final reviewed fixture verification

The four-file reviewed test variant now passes **all106 named RK1/RK1K/continuation cases** (`tagged-final3`) and **9/9 host normal +9/9 host race** (`host-final2`). `accepted-conditional-summary.json` now selects these final exact runs and the13 named recovery/classifier tests. Earlier green variants remain historical, not the final evidence. Final source hashes cover all four test files; production bytes are unchanged. Only coordinated performance remains pending.

## Performance first current-candidate run — FAIL retained

Parent stopped its owned API/Web/renderer and all other task DB matrices were terminal. Fresh database `core_20261108_i1_p95_61533`; no race; original test files/limits/samples. Preflight CPU10, load average9.36, PG39connections, other recorded task databases0connections. External background load remains a measurement limitation.

- I1 detail P95 **67.512ms**, preview **435.163ms**, closure **1710.239ms**, all <2000ms: PASS.
- Disable51plans/1530members: preview P95 **718.007ms** <3000; apply P95 **4083.550ms** >3000, max4357.256ms: FAIL.
- Triage10kformal/2kpending: list **68.878ms**, filter **118.422ms**, action **18.225ms**, all <2000: PASS.

Disable slow samples cluster5–7 (3455.683/4083.550/4357.256ms), then drop to1028.614ms atsample8 and mostly ~1100ms thereafter. This is evidence for investigation, not permission to discard samples/change warmups. A separate private diagnostic preserves35cycles and business assertions, recording actual pgx statement/batch timings and automatic-analyze counters without SQL changes or manual ANALYZE. Original acceptance remains failed.

## Performance repairs and current final product gate

Actual SQL diagnostics showed1530 individual running-task reads plus per-issue source/target participation-lock round trips. The first repair batches running counts for the exact authorized workspace/issue set, counting only running status. It reduced disable apply P95 from4083.550ms to969.367ms in run2, but closure still failed at2041.992ms>2000. The second repair pipelines the original participation SELECT FOR UPDATE statements in unchanged issue/source-before-target order, consumes every result and Close error, retains ErrNoRows semantics and all validation before writes, and preserves the single-issue query path. No migrations, new dependencies, altered warmups, sample counts or thresholds.

Regressions: original3 task reads versus required1 failed, then status2/1/0, unrelated/foreign workspace, empty selection passed. Original3 participation reads/0 batches versus required0/1 failed; final68 iteration top-level/100 named race cases PASS including missing/inactive source, missing/historical/current target, duplicates, empty sets, no writes during failed preparation, SQL parity and existing lifecycle/concurrency/rollback tests. Both generated additions are canonical sqlc1.31.1 output; all130 generated files regenerate identically. Vet passes. Independent reviews approved both narrow changes.

**Performance run3: all3 original tests PASS**. I1 detail93.670ms, preview124.230ms, closure1662.866ms (<2000). Disable preview571.568ms, apply1937.854ms (<3000). Triage list133.460ms, filter64.665ms, action13.987ms (<2000). Fresh DB, no race, coordinated quiet task window; all original failed samples remain. Product hashes frozen in `product-final-hashes.json` (8 implementation/query/generated files plus regression file).

Final full-package regression is live in `product-final/result.json`, exec46572: fresh service and handler databases, serial race runs with90-minute aggregate harness budget, then internal/iteration. Previous default82-package results apply to the earlier product bytes until affected complete packages are replaced by these new results. Parent rebuilds API/bundled CLI and runs final browser acceptance.

## Final backend acceptance (current frozen product)

Complete fresh package runs finished: service **1455 top-level /3453 named PASS**, handler **2888 top-level /5383 named PASS +7 known skips**, iteration **38 top-level /122 named PASS**. Zero failures. Frozen implementation and regression hashes still match exactly. Selected82-package inventory now uses these complete runs and reports **10459 top-level PASS /20 SKIP**, **21241 named PASS /21 SKIP**. Performance run3 remains **3/3 PASS** on this same implementation. No backend test process is pending.

Eight product implementation/query/generated files and the new regression are recorded in `product-final-hashes.json`; four earlier tagged fixture files remain separately hash-bound. The only additional spec edit is `.trellis/spec/server/iterations.md`. No deployment, paid-provider account, shared database cleanup or global configuration changes occurred.

## Final resource and source audit

`final-source-audit-v3.json` records the final9 modified tracked server files and4 new files (8 product implementation/query/generated +1 service regression +4 tagged fixture/classifier files total). All product-freeze hashes match. `go vet -p 2 ./...` on the final product passed (`vet-final-product.log`, empty). All80 recorded databases have zero connections; retained for diagnosis. Final cleanup found two Redis containers left by earlier compile failures occurring before runner cleanup registration; their recorded owner PIDs were absent, then both were stopped and all13 task-labelled inactive containers removed. No task-owned backend container remains. Original failures/logs/databases stay intact.
