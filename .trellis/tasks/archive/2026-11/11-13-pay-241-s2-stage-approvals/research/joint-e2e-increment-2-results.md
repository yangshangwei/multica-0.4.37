# S2 joint E2E authoring — increment 2

Owner: `/root/s2_e2e`. Status: authoring complete, targeted static and fake-provider unit checks passed; no live API, database, browser, daemon or model execution. Date: 2026-11-14.

## Change boundary

A05 needs an actual prepared Git source, a human C1 comment, a controlled provider invoking CLI T1/T2/T3, protected T4 UI reads, and distinct QA decisions. The existing preparation-record fixture seeds retained history and cannot establish this path. The existing host provider/daemon contract can be extended without a second runner.

The coordinator approved this bounded ownership expansion:

- Existing new `e2e/collaboration-stage-approvals.spec.ts` and `e2e/fixtures/collaboration-stage-approvals.ts`: real local Git fixture, typed comment/report/QA journey and evidence.
- `e2e/fixtures/collaboration-host-runtime.ts`: optional validated Git config, test-report actions and optional failure release. Existing success defaults and deadlines remain.
- `e2e/fixtures/collaboration-host-provider.go` and its `_test.go`: deterministic prepared-marker observation, actual run-local CLI subject/submit/receipt, and the optional ordinary-provider failure result. This does not change the separate multi-account provider's callback contract.

The same server owner must separately add optional `git` to `server/internal/daemon/collaboration_host_router_test.go` before capability registration. The requested wire shape is the existing HostGitConfig: `executable`, `bundle_file`, `bundle_digest`, `size_bytes`, `object_format`, `commit_oid`. Readiness exposes only the public capability Git summary. This agent does not edit server code.

Increment 1 source and reports were verified unchanged and snapshotted under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-e2e-increment2-20261114T075324Z/`. Its prior conclusions remain historical. Static inspection also found that its execution-count queries used an absent issue_id on command/run; this increment corrects the owned fixture to join collaboration_session. No original runtime pass is being rewritten.

The new flow keeps source-only `artifact_digest:null`, runtime-reported results, actual execution outcome and human QA approval separate. Inline report logs keep their hashes and array indices without fabricated checkpoint IDs. A bare host run must report subject unavailable. The first prepared run retains failed cases and settled failure; a distinct explicit run records passing cases and settles successfully. No report, preparation, human decision or membership row is seeded. Release/rollback/M1/admin acceptance stays outside this increment.

## Authored flow

1. Build two real commits in a new task-private local Git repository using an explicit `COLLABORATION_STAGE_GIT` executable, empty template and disabled global/system configuration and hooks. The synthetic PR heads use those actual commits. A bundle retains the committed `stage-regression.json`; dirty and untracked controls are added afterward and must not appear in the provider's prepared directory. This construction is authored but was not executed by this agent.
2. After the earlier UI journey reaches QA, the creator explicitly switches the issue from manual to assisted. The QA actor creates its own execution workspace. Resource offering, adoption and owner consent use the existing APIs/shared UI; the runtime's Git capability is registered before consent.
3. A separate bare Host resource runs an ordinary typed comment. Actual CLI `test-report subject` must return a recognized unavailable result; no report is created and successful bare execution cannot enable QA. That daemon closes before the prepared one starts.
4. `Prepare test input` must change the existing comment draft without a POST. The existing typed mention preview and explicit send produce C1 with the exact candidate reference. The real claim/Host preparation must deliver the committed marker, exclude the dirty/untracked controls and create the actual command/run identity.
5. The controlled provider reads the prepared marker and runs a tiny deterministic duplicate-callback case. The first action injects duplicate effects and submits two bounded cases/inline logs through actual CLI subject/submit/receipt. All three commands, original key, ACK, input/output hashes and source file hash are retained. T4 must match the actual provider attempt in the daemon ledger, initially report execution as unsettled, then preserve its report digest after the ordinary provider error result settles the run as failed.
6. Shared Web report UI displays the original failure and logs. Source-denied report reads return 404. Direct approval of that failed report is rejected; the configured QA actor records an explicit return. An explicit stage rework and fresh code review precede the next C1 request.
7. A new run and request submit the passing capture and settle successfully. The old failure's ID/digest/subject/cases/logs remain identical, and the new runtime report leaves the earlier QA return intact. The shared Desktop report reader opens the passing report. QA selects its full T4 detail and exact preceding approval, explicitly approves, and the Web action enters Release. No build, deployment or release approval is inferred from the source-only report.

The prepared and bare resources reuse the reviewed S5 `startRuntimeHeartbeat` and `runtimeAvailability` functions: managed-owner identity, one bounded request, 15-second interval, 10-second request bound, sticky errors and pre-trigger freshness checks. Product freshness remains 150 seconds; the Host poller remains five minutes and execution remains three minutes. Cleanup closes every owned daemon before joining heartbeats and restoring the exact recorded gate booleans with a monotonic epoch. Browser contexts, the independent Electron fixture and SQL connection are closed afterward. Database/history and the local Git fixture are retained.

## Canonical fixture compatibility

- Existing Host success release remains `r`; optional failure release uses `f` only with `testReport:true`.
- The selected deterministic Host provider must answer `--fixture-capabilities` with the new report/failure contract. The separate multi-account provider shares the old fixture-kind string but lacks this new handshake, so it cannot be silently substituted. A failed early handshake cleans only the newly created fixture directory.
- Git options are optional. Node validates absolute regular paths, declared bounds, actual bundle size and digest before forwarding. The server owner's optional `git` field and safe `ready.git` summary match the existing HostGitConfig. The actual product Host preparer remains responsible for materialization and launch accounting.
- The report action reads at most 16 KiB of marker data and captures at most 64 KiB per CLI output stream under one 20-second command sequence. Marker fields and fault choice are closed. Source absence is distinct from an accepted report. The ordinary provider's result remains a separate success/error message.

## Verification performed

| Check | Result | Raw evidence |
| --- | --- | --- |
| New provider action red tests | 2 top-level / 4 named failures: unsupported action | `provider-red-1.log` |
| CLI output-bound red test | 1 FAIL: promoted ReaderFrom bypassed the writer limit | `provider-output-red-1.log` |
| Final standalone Host provider unit suite, `-race` | 8 top-level / 31 named PASS, zero FAIL/SKIP | `provider-green-2.log` |
| Existing Host cleanup portability tests | 4 PASS, zero FAIL/SKIP | `host-node-1.log` |
| Targeted strict TypeScript | Exit 0, no diagnostics | `typecheck-4.log` |
| Targeted ESLint on the three TS files | Exit 0, zero warnings | `lint-4.log` |
| Playwright discovery only | Exit 0, one scenario in one file, no test body executed | `discovery-2.log` |
| Final source bindings | TS files match final static checks; Go files match final provider checks | `final-verification.json`, `provider-final-verification.json` |

Provider unit tests invoke only the exact test executable as a fake CLI, with private HOME/TMP and no inherited database/model credentials. They do not exercise a real CLI/API/run or prove Git preparation. The server owner separately tested its optional Git harness input; its source hashes are captured as dependencies, not claimed as tests by this agent.

Preserved authoring corrections: the first TypeScript pass used `target.queue_state` instead of the existing `target.control.queue_state` and did not guard an optional launch inventory; a later lint pass rejected an explicit throw in `finally`, which now accumulates cleanup failure while preserving the other cleanup steps. The initial stage fixture count queries were corrected from absent issue_id columns to session joins. A final self-review found that embedding `bytes.Buffer` promoted `ReaderFrom`, so `io.Copy` could bypass the new writer's 64 KiB bound. The dedicated regression reproduced that failure; a private buffer now leaves all pipe copies subject to `Write`, and the final 31-test race run passes. None of these was hidden by changing product timeouts, connection pools or cleanup rules. Discovery retains the existing pnpm configuration warning and dotenv banner.

Logs, original snapshot, check configs and the final frozen five-file source are under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-e2e-increment2-20261114T075324Z/`. `joint-e2e-increment-2-results.json` contains exact file/log/config hashes, commands and named test terminals. Increment 1 reports and snapshot remain unchanged. No dependency or Git-index change was made.

## Remaining acceptance

**A05 and all full A01–A12 cases remain NOT_RUN.** Root must bind the final post-2d API/CLI/production Web/Desktop and the newly built deterministic Host provider/daemon, supply the explicit Git executable, verify the outbound-mail-disabled environment and execute on a fresh task database. Keep real GitHub refresh credentials disabled for this synthetic S2 API; real connector evidence belongs to S4. The E2E now requires the base Host provider with report capability, not the closure provider binary used by other scenarios.

Actual Git helper attempts, C1 first-claim binding, runtime report submission, failure settlement, UI selectors, cross-workspace resource consent and teardown have not been executed here. Retain the first configured run and fix genuine integration failures under the appropriate owner. Add release/rollback facts, build-bound approval and all M1/admin/final-write paths only after their served contracts stabilize; this source-only QA flow cannot substitute for those or for real S4 business evidence.
