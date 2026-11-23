# Execution checklist

## 1. Planning and review

- [x] User consent to create task and advance testing.
- [x] Load project/workflow/spec context; preserve existing untracked task.
- [x] Pin branch, HEAD and local main merge-base; inspect prior failed acceptance and recent changes.
- [x] Collect current E2E tests without executing business tests: 439 / 148 files.
- [x] Write PRD, design, functional plan, context manifests and handoff scaffold.
- [x] User reviewed concrete planning artifacts: “通过计划 启动执行”. Independent planning review: PASS WITH NOTES; notes applied to execution permits.
- [x] Task started after review: status is `in_progress`.

## 2. B0: execution permit

- [x] Record actual source status/fingerprint, tools and baseline. A detached same-commit checkout is preferred if another session can mutate source or `.next`.
- [x] Add `/opt/homebrew/opt/libpq/bin` to the task command PATH; verify psql locally.
- [x] Inspect aggregate PG connection usage read-only; record cluster headroom and every owned pool.
- [x] Allocate task-owned private registry/config/ports and fresh DBs, verify actual identities before migrations/tests. Do not use the mismatched default environment.
- [x] Preserve all historical failures and the separate old private launcher review block. Do not reuse it without completed review.
- [x] Create a run ledger before each command; keep credentials/private configs outside tracked task artifacts.

## 3. B1: static, unit and integration

Run from the pinned task checkout with stdout/stderr and exit status saved per command. These commands do not require browser services:

```sh
pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
pnpm check:ui-exports
pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2
bash scripts/test-go.test.sh
bash scripts/dev-env.test.sh
bash scripts/check.test.sh
bash scripts/check-ui-wildcard-exports.test.sh
node --test scripts/check-collaboration-native-tests.test.mjs
pnpm test:seed
go -C server build ./...
go -C server vet -p 2 ./...
```

- [x] Run static/TS/script checks; account for all workspace scripts actually present, not a synthetic global count.
- [x] Run Go race suites with the repository Agent CLI guard and fresh migrated DB per DB-backed package. Check the package's `TestMain` before classifying it as DB-free; `-run '^$'` still executes `TestMain`.
- [ ] Canonical all-Go entry is `bash scripts/test-go.sh --race`; it serializes packages but shares a URL. For campaign acceptance use a reviewed per-package isolation permit/runner rather than silently running that entry against the default or shared browser DB.
- [x] Go test JSON distinguishes top-level, subtest, package, skip and no-test/compile-only counts. Retain skips and actual reasons.
- [x] Run Mobile separately because root scripts exclude it:

```sh
pnpm --filter @multica/mobile typecheck
pnpm --filter @multica/mobile lint
pnpm --filter @multica/mobile exec vitest run --maxWorkers=2
bash apps/mobile/scripts/ios-run.test.sh
```

The Mobile package test script chains Vitest with a shell regression. Execute those two existing commands separately to bound Vitest workers without accidentally forwarding flags to the shell regression. Unit success is not simulator acceptance.

## 4. B2: legacy production Web

- [x] Create/build/start owned API and production Web with documented registry commands and provenance; record build ID and rendered API/WS addresses.
- [x] Build matching desktop renderer/preload/CLI if required; guard Agent CLI discovery.
- [x] First run core journeys, task lifecycle, issue mutation branches, workspace access, project/iteration and notifications as a smoke gate.
- [x] Then execute the complete applicable inventory with one worker and zero retries; keep skipped identities for B3 reconciliation.

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- pnpm exec playwright test --workers=1 --retries=0 --reporter=line,json
```

Set `PLAYWRIGHT_JSON_OUTPUT_NAME` to the task run's private/sanitized result path and `--output` to its artifact directory. Supply only the permit's verified URLs/DB/configuration; the config does not start services. The repository `make check` is an ordered reference pipeline, not a substitute for missing conditions. The UI-export regression temporarily writes probe source files; run it serially in the owned checkout and confirm product status is clean before source fingerprints/builds.

## 5. B3: conditional lanes and high-risk replays

- [x] Stop owned legacy API when idle; record connections before starting each next lane.
- [x] Password/admin: synthetic accounts, explicit limiter mode, device/cloud disabled, managed-installation/admin flags only as required. Verify live `/api/config` and CSRF cookies from actual login.
- [x] Device: separate effective startup config and live proof; do not toggle a source deployment config.
- [x] Host/namespace: exact fake executables, owned workdirs, execution-mode-specific gates, same-source artifacts.
- [x] Real Electron: same-source renderer/preload; exact origins; task userData; run original focus assertions unchanged.
- [x] Re-run Transfer journeys and all 15 old acceptance failures against current code; preserve the old report without updating its results.
- [x] Re-run memory startup-vs-live invariant, followed by memory nested-runtime -> Leave on the same API/DB without resetting the gate between the two.
- [x] Re-run knowledge correction/withdrawal physical-input proof and deployed switch OFF/ON on dedicated instances. Independent review of inherited adapter-v5/RKI6 results is a prerequisite for reusing their private runner.
- [x] Reconcile every original 439 identity to final status; dynamically registered cases and new manual tests get distinct IDs rather than silently changing the denominator.

## 6. B4: platform/installation/compatibility

- [ ] Mobile unit checks plus simulator availability and manual core semantic-parity matrix.
- [x] macOS full Main installation/runtime journey if matching local packaged bytes exist; distinguish fixture runner from full Main.
- [ ] Linux native/installed/deb and Windows native/install/upgrade only with their actual prerequisites. Do not infer them from compilation or a Linux daemon container.
  - [x] Linux ARM64 DEB package-manager lane: current candidate archive verified; actual baseline-to-candidate dpkg upgrade, independent fresh installation and remove/reinstall/purge passed in isolated containers. Evidence: `evidence/resume-linux-deb-install-1/`.
  - [ ] Actual native Linux desktop session and other package formats/architectures remain separate open gates.
- [x] Real commercial Provider calls remain BLOCKED until exact accounts/model/call cap and budget are authorized. Prior experimental budgets are not reusable authorization.
- [ ] Verify fresh migrations, conditional migration behavior and old-client response parsing with existing focused tests; rollback/destructive migration experiments run only on disposable owned databases.
- [x] Browser exploration checks error/empty states, narrow layout, Chinese text, keyboard/focus and console/network errors; record screenshots for reproducible UI issues.

## 7. B5: close or hand off honestly

- [x] Independently review coverage and raw evidence, including all skips/setup failures and case counts.
- [x] Update defects, report, progress and session prompt; list changed files and remaining risks.
- [x] Restore owned gate booleans; prove owned process shutdown without stopping other sessions. Retain DB/evidence for diagnosis.
- [x] No full-acceptance claim while mandatory gates are FAIL/BLOCKED/NOT_RUN. Keep task in progress when actual execution remains.
- [x] No commit, remote push, merge or deployment is needed for this test-planning handoff. If later committing artifacts, use Lore trailers and stage only this task's intended files.

## User-directed continuation plan (2026-11-02)

- [x] Preserve original P95 failure; analyze samples/code and one separately named closure diagnostic on a new owned database.
- [x] Verify macOS distribution/file replacement, then actual immutable old/new Main configuration/preference/localStorage retention with protocol capture and cleanup.
- [x] Prepare exact native Linux, Windows, iOS, trust and Provider execution handoff in research/remaining-platform-handoff-1.md.
- [x] Independently review new evidence and complete final cleanup/document reconciliation (research/next-evidence-review-1.md including runtime2 supplement; evidence/next-cleanup-2/run.json). Both review notes resolved; original fixture limitation retained.
- [x] Canonical SQL plan/transport batch: research/sql-diagnostic-batch-plan-1.md; new DB + additive overlay; three 1,000-statement write replays and 3,000 canonical reads, rollback/result checks; original P95 not rerun. See research/perf-sql-plan-diagnosis-1.md.
- [x] Independent diagnostic evidence review PASS WITH NOTES (research/sql-evidence-review-1.md/.json); initial child429 records preserved, all6000plans and5900historical files checked.
- [x] Fresh DB-free membership SQL equivalence check, read-only local platform/protocol prerequisites, and owned resources/connections cleanup (evidence/next-sql-cleanup-1/run.json).
- [ ] Full acceptance remains open: original performanceFAIL, real authenticated upgrade, automatic updater, remaining native platforms/trust/provider gates.

## Prepared-plan and read-semantics continuation

- [x] research/cached-diagnostic-batch-plan-1.md; preserve5,949historicalfiles and allsource/frozenidentities.
- [x] Native prepared/statistics run1 on freshDB,6,000actualplans; native typed/readsemantics+currentowner focusedrace on2freshDBs,12top+14subPASS.
- [x] Coordinator newrun2/no-stat DeallocateAll/reprepare discriminant after2empty nativefollowups;8,000actualplans,4fullrollbacks; research/perf-cached-reprepare-diagnosis-1.md.
- [x] New localplatformreadonlypreflight2 and finalcleanup2:9DBconnections0,process/port/mount/container/protocolidentity verified.
- [x] Independent cached/semantics evidence review PASS WITH NOTES including final documentation/cleanup supplement; coordinator historical integrity check passed.
- [ ] Required originalP95/platform/auth/updater/trust/Provider gates remain open; no completeacceptance/archive.

## Remaining-gate continuation batch

- [x] Preserve6,113taskfiles/36workspacefiles, restore existing session task pointer, and record research/remaining-gates-batch-plan-1.md.
- [x] Native platform prerequisite slice:10commands/4pinnedartifacts; no new runnable gate.
- [x] Native identity/symptom slice:3source/frozenidentities,158rawhashchecks; no new genuine symptom; private checker correction retained.
- [x] Separate coordinator verification and fresh cleanup:56ownedDBconnections0, sixportsclosed, nativeprotocol/process/mount/containerchecksPASS.
- [x] Update progress/report/defects/matrix/sessionprompt/task metadata without product/SQL/index/dependency changes or repeated passed gates.
- [x] Final independent current-document/history/cleanup review PASS WITH NOTES (research/remaining-final-coordination-review-1.md/.json) and final coordinator hash reconciliation.
- [ ] Required original performance/native/auth/updater/trust/Provider acceptance gates remain open; task staysin_progress/noarchive.

## Webhook assertion and updater readiness continuation

- [x] Preserve6,234taskfiles/36workspacefiles and all source/frozen identities; research/readiness-batch-plan-1.md.
- [x] Native research found CLI preservation unimplemented; retain originalskip and avoid duplicate unsupported loss matrix.
- [x] Coordinator fresh ownedDB/template/budget allocation and concrete Webhookpermit.
- [x] Native private canonical Webhook lookup diagnostic:onecompile/onceraceexecution,1top+4subPASS; actualPrepareError42P01/clientclosed/404/429/debt/no-body/no-write witnesses.
- [x] Native Mac updater offlinefeed/manifest/profile/future recipe and source/cache/native-stage boundaries;5commandsPASS preparation only.
- [x] Coordinator raw/terminal/semantic checks; append-only tested verification specs.
- [x] Independent code/raw review and narrow overlay Go vet PASS; fresh57DB/resource cleanup and separate final document review PASS WITH NOTES; coordinator hash reconciliation complete.
- [ ] Full mandatory performance/native/auth/updater/trust/Provider gates remain open; in_progress/noarchive.
