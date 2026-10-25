# Implementation evidence

## 2026-10-25 — activation and scope

- The user explicitly authorized implementation of the existing reviewed plan. `task.py start` activated this task on `codex/cross-workspace-collaboration`; both context manifests passed validation (8 entries each).
- Existing planning/spec edits are preserved. Unrelated `apps/web/next-env.d.ts`, `docs/multica-introduction.html` and the archived introduction task remain outside this change.
- Behavior gap: collaboration currently requires one Linux namespace attempt. The new host mode must reuse ordinary Provider execution while retaining issue authority, immutable authors, separate sessions/directories, per-launch permits and unknown-execution barriers.
- Change ownership: protocol/service/SQL and compatibility; common Provider lifecycle observation; shared core/views; daemon ledger, preparation and runner integration. No ordinary task business-route bypass, new dependencies, native sandbox or selector redesign.
- Regression strategy: strict v2/v3 and v5/v6 boundary matrices; real test-owned subprocess start/wait/cancel/retry cases; durable restart/replay and session ownership tests; DB-backed control/queue/squad tests; shared UI parsing/copy and full repository checks.
- Native platform and Provider acceptance remains unproven until actual evidence is recorded. Compilation or a fake-provider success alone does not establish a Provider/version/platform rollout.

## Stage status

| Stage | Status | Evidence |
| --- | --- | --- |
| S1 contracts and migration | Implemented; reviewed | Protocol/DB/HTTP strict version, original identity and independent receipt tests pass |
| S2 ledger and lifecycle | Implemented; reviewed | Native macOS race tests; Linux arm64 host suite; Windows binaries compile |
| S3 environment and Provider adapter | Implemented; reviewed | Ordinary Backend, independent sessions/files and every Git helper tested; selected CLI binding and absent Provider result reviewed |
| S4 control, queues and squads | Implemented; reviewed | Real DB capacity-one, stop/revoke, explicit Resume and namespace-drain cutover tests pass |
| S5 consent, clients and copy | Implemented; reviewed | Core/Views and full repository TS checks pass; actual Web/native-renderer and Linux full Main host journeys pass; banner correction verified |
| S6 native acceptance and finish | In progress | Native CI matrix added; Windows, authenticated Provider and packaged acceptance remain explicit gates |

## 2026-10-26 — verification and review

- Full Core and Views suites passed (4,126 and 7,002 tests); repository `pnpm typecheck`, `pnpm lint` and `pnpm test --concurrency=1 -- --maxWorkers=2` passed. Existing unrelated lint warnings remain.
- Full guarded `pkg/agent` tests and lifecycle race checks passed on macOS. Full `collaborationrun` race suite passed after retained-claim, interrupted-response, preparation-fact, outcome/proof separation and late-callback fixes.
- Real DB suites verify owner/author identity, strict old/new modes, per-attempt and final termination, unissued nonce tombstones, Stop/revoke, exact outcome ACK recovery, capacity-one squads, mixed comments and the cutover refusal of new namespace roots.
- Linux arm64 runs test-owned Provider/Git processes as non-root in a native Linux container. The earlier root run correctly hit the ordinary Claude privilege guard; no guard was weakened. Docker bind mounts could not resolve host paths, so test binaries were copied into task-owned containers instead.
- The final broad `trellis-check` completed with no unresolved production defect. Its selected-CLI binding, version normalization, missing-business-result, consent-metadata, read-negotiation and credential-redaction findings were fixed and covered by regressions.
- The first two `make check` runs failed and ended. The first exposed fixed contract/guard issues. The second exposed interference between DB-backed packages sharing the organization singleton. A controlled 12-iteration comparison failed with package parallelism 2 and passed with package parallelism 1; `scripts/test-go.sh` now serializes packages while retaining within-package parallelism 2. A fresh full check is required after the focused QA services stop.
- Current evidence and remaining platform gates are tracked in [verification.md](verification.md). Native Windows and real Provider/install acceptance are not claimed from cross-compilation.

## 2026-10-26 — composed acceptance and final checks

- The native macOS Web/Electron journey passed with four actual test-owned Provider launches: shared-Agent execution by another participant, a scoped Stop, an independent issue/session/directory, and explicit Resume of mixed-author waiting input. The harness checks current CLI hashes, attachments, original authors, v3 receipts, cleanup facts and capacity release.
- The composed test found the installed macOS name `macos` differs from native GOOS `darwin`. A single boundary mapping now serves registration and subsequent admission; other OS mismatches remain rejected. The real DB matrix and composed rerun passed.
- The final Linux arm64 non-root host lifecycle/runner rerun passed after review fixes. No user account or installed Agent CLI was used.
- A remaining v6 execution-panel banner incorrectly claimed execution was unavailable while its gate was enabled. The focused regression failed before the conditional-copy fix; 11 panel/host-display tests and scoped lint now pass. Legacy v5 wording is retained, and disabled v6 dispatch preserves existing execution facts and Stop controls. Production Web/Desktop rebuild and final actual E2E rerun passed (1 test, 25.7s); 19 screenshots passed visual review.
- The native verifier now rejects skipped required subtests even when their parent reports pass, and rejects missing/unsupported runner OS metadata. Seven script regressions pass. Both native macOS arm64 toolchains (Go 1.26.9 and 1.27.0) passed 46 top-level tests and 62 subtests with no skips; all 11 required cases passed the corrected gate.
- The third `make check` is running under Go 1.26.9 in its fresh `check-20261025192655-82819` environment. All 15 static tasks and all five TypeScript suites have passed: Core 4,147, Views 7,030, Web 285, Desktop 1,235, docs 62. Go race tests are in progress. The prior focused password API/renderer are retained; its Web was stopped before the full check.
- A current Linux ARM64 full Main package is being built in a separate detached worktree. Existing root Desktop outputs and the Oct 22 package volume are preserved. A temporary native CI snapshot is being prepared locally; nothing has been pushed and no real Provider account has been accessed.

## 2026-10-26 — current package and remaining repository failure

- The current Linux arm64 directory package built with Go 1.26.9 and Electron 39.8.7; frozen dependency installation reused 1,389 packages without downloads or lockfile changes. Native package hashes and bundled CLI version passed. Full Main startup then exposed an obsolete test-only `isCliInstalled()` call; the explicit v6 fixture now observes current real `getCliPath()` IPC, while legacy fixture defaults remain unchanged.
- The actual full Main host journey passed first run (1 test, 22.0s). Linux packaged Main/preload/renderer used a real synthetic password form to control the macOS fixture runtime. Three real launches verified target Stop, restart while paused, mixed-author explicit Resume, original history/receipts, CLI/attachment hashes and capacity. Screenshots passed review. No real Provider account was used.
- The third full check completed all static, TS, Go race and Go vet stages. Handler passed in 259.996s, service in 791.747s, agent in 325.459s. Browser result: 260 passed, 128 environment-gated skips, one invitation navigation timeout. The run cleaned its own services and Go database.
- The invitation failure showed successful acceptance and a redirecting card but no route change within the existing 5s assertion. Original backend acceptance, onboarding, user refresh and both workspace reads were fast 200 responses. Ten traced repeats passed unchanged; simple query invalidation also did not reproduce failure. Investigation is bounded and no speculative timeout/product repair has been made.
- Native Windows CI is prepared as local snapshot `92bb6d86` with a branch-only workflow. Publication to the existing public repository is awaiting explicit approval; root branch/index were preserved. The earlier real Provider account question also remains unanswered.
- The invitation investigation ended after 11 unchanged canonical traced passes and eight passive no-trace passes of the original five-second assertion. Navigation completed in 1.859–1.867s; no page errors occurred. No production/test timeout change was justified. A new complete canonical check is running via `scripts/check.sh --trace=retain-on-failure` in environment `check-20261025210127-31684`, so any recurring full-suite failure retains the evidence absent from the original run.
- Final canonical check passed with exit 0. All static, TypeScript, Go race/vet and script checks passed; browser result was 261 passed and 128 environment-gated skips in 13.9m. The unchanged invitation case passed in 2.8s. The run stopped its Web/API and dropped its isolated Go DB successfully. The original single timeout is retained as unexplained history, not claimed fixed.
- Implementation and acceptance code are committed locally as `75d4d341c037`, `e4df692885c7`, `8bfc84ad17fb`, `88d9949b1f06` and `a76605caca6c`. Only task-owned files were staged; the 11 parallel/generated dirty paths were excluded. These work commits do not publish or enable host execution. Final contracts/evidence are committed with the task documentation; the task remains in progress for native Windows/account/remaining delivery gates.
