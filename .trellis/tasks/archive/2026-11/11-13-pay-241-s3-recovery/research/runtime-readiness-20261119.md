# Research: Extras-B controlled runtime readiness

- Query: Can the authored four-top/ten-subcase suite run against the accepted blocking candidate, and what must the coordinator bind before execution?
- Scope: internal; read-only source and retained evidence inspection.
- Date: 2026-11-19
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Result: **Conditionally runnable; actual Extras-B remains NOT_RUN.** The two authored files retain their accepted author seals. No definite production or scenario defect was established in this inspection. A coherent final harness/source binding, three explicit runtime executables, a fresh dedicated database and a new closed runtime configuration are still required.

## Findings

### 1. Files and actual source identity

| File or retained input | Purpose / observed identity |
| --- | --- |
| `e2e/collaboration-recovery-runtime.spec.ts:10` | Registers Web/Desktop × notifications/withdrawal; each top has 480-second budget and explicit environment opt-in. Current SHA-256 `3ad9fe6cab428dd5043c34c30f7f07b79cc42d5efdff978343fc5ad2bbcd1ef8`, exactly the R03 author seal. |
| `e2e/fixtures/collaboration-recovery-runtime.ts:51` | Current authored composed fixture. SHA-256 `81d9fd6ffff94deb5b400405080377eed4d5959e9d5bbe0defd4166b5219bf9d`, exactly the R03 author seal. |
| `e2e/fixtures/collaboration-host-runtime.ts:104` | Closed host-environment checks, real enrollment, callback owner, daemon restart and ledger cleanup. Current SHA-256 `3ea2e6a51cd782f55d2268427739e6749f93c8bf23069f817147f2cff603fc92`; identical in ROOT, client R05 and author R03 harness. |
| `e2e/fixtures/collaboration-host-provider.go:26` | Stdlib-only controlled Claude stream-json Provider; current SHA-256 `fc1095dc4fea11ad6a3e0a96647d99996687ba10937c00ebda53097f26fcd77f`, identical in those three locations. |
| `server/internal/daemon/collaboration_host_router_test.go:55` | Guarded native poller entry, not a Go test body. Current SHA-256 `6ea88ebb555918c2e75878dca7addbc0af11adc5dadb6604fb8fad230fa70aab`, identical to accepted server R05. |
| `e2e/fixtures/collaboration-recovery.ts:33` | Shared current-client observer, session and actual WebSocket reconnect owner. ROOT and author R03 SHA-256 `7f125d331adcfa85dab491b3def6ec07bca084238b71eae90f8e5eafcbffb3d1`; client R05 retains an earlier helper (`a4adc002b2080f2e046e06662a8efbfad5bbade225c52c5a5593490638e3fa01`). |
| `e2e/fixtures/collaboration-native.ts:18` | Real Electron renderer/preload fixture; ROOT, client R05 and author R03 match SHA-256 `b4038d1ba4eeb3e47c3d9730326d2dcb7f41af07e929bd84013712a5775373f1`. |

The accepted block R08 deployment used two actual source trees under
`/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`:

- Server: `source-block-work-r05`, 5,077-path manifest at `verification/block-package-continuation-r05/handler_race/source-before.json`, SHA-256 `02069a51d522f3b036279dee337b18c684ab6a0fe46eb57312922ffeeb4d022a`; API binary SHA-256 `fc22518dc14d0a16e54d54bccfd04423f3d206064b695caeda71d9c247e6a898`.
- Client: `source-block-ui-run-r05`, manifest `verification/block-ui-run-r05/source-manifest.json`, SHA-256 `5833741fbbe92f0bfd82dd2b499235d4d361e7179e04c0c26f339305f31450e7`.

The prospective `source-block-integrated-r03` named in the old author intake does **not** exist. Do not use its name as proof of a combined source. Author R03 static checks used the `source-r01` package graph. The coordinator must assemble and hash a current harness against the actual accepted package trees and explicitly overlay the current shared helper, runtime files and bounded registration fixture. The earlier client R05 helper lacks exported `RecoveryActor`/`RecoverySurface`, exported `withSessions`, and the current observer/cleanup API consumed by Extras-B; copying only the two runtime files into that earlier helper graph is insufficient.

### 2. Exact runtime prerequisites

`recoveryRuntimeEnvironmentReady()` combines the ordinary S3 environment gate with `PAY241_S3_RECOVERY_RUNTIME_ACCEPTANCE=1` and explicit runtime executable keys. The following closed configuration is required before any test body:

- `PAY241_S3_RECOVERY_ACCEPTANCE=1`, `PAY241_S3_RECOVERY_RUNTIME_ACCEPTANCE=1`, `E2E_PASSWORD_AUTH=1`.
- Exact `DATABASE_URL` and `TASK_DATABASE_NAME`; loopback host and dedicated nondefault database. The URL name is checked before setup; `audit()` later checks `current_database()`.
- Identical API origin in `COLLABORATION_E2E_API_URL` and `NEXT_PUBLIC_API_URL`; actual API deployment equals `MULTICA_DEPLOYMENT_ID`.
- `PLAYWRIGHT_BASE_URL` for production Web, `COLLABORATION_ELECTRON_RENDERER_URL` and `COLLABORATION_ELECTRON_SOURCE_ROOT` for the matching native fixture, preload and source graph.
- Absolute existing executable files for `COLLABORATION_HOST_DAEMON`, `COLLABORATION_HOST_PROVIDER` and `COLLABORATION_HOST_CLI`.
- Task-private `HOME`, temporary roots and `PAY241_S3_PROFILE_ROOT`; actual executable permission/provenance must be checked by the coordinator because `hostEnvironment()` itself tests existence/file type, not executable mode.
- API password/managed-installation startup configuration, allowed origins and bounded pool budget; `/api/config` must return password mode, managed-installation support and the expected deployment.

No JSON under the retained root's `configs/` contained any of the four runtime-specific keys at inspection. Its top-level `build/` had no Extras-B daemon/Provider/CLI binaries. This establishes missing bindings in this retained allocation, not absence of binaries or environments elsewhere on the machine.

Build from the newly frozen accepted server/harness graph, preserving full argv, tool/source/configuration hashes and output files:

```text
go -C <accepted-server-root>/server test -c -o <private-build>/s3-host-daemon.test ./internal/daemon
go -C <accepted-server-root>/server build -o <private-build>/s3-host-cli ./cmd/multica
go build -o <private-build>/s3-host-provider <frozen-harness>/e2e/fixtures/collaboration-host-provider.go
```

`go test -c` compiles without TestMain. The Provider file is stdlib-only. Its explicit harmless readiness commands are `--fixture-kind` and `--fixture-capabilities`; required results are `multica-host-e2e-provider-v1` and `{test_report:true,failure_release:true,prepared_git_marker:true}`. No installed agent lookup or real account is needed.

The actual daemon argv is `[<s3-host-daemon.test>, "__multica_collaboration_host_router_poller"]`. Its `init` guard consumes the issued managed configuration on stdin and exits before normal test execution. The selected lane advertises protocol 3, `host_process_supervision_v1`, preparation 2, v6 human reads and one actual machine slot. Do not invoke a differently named acceptance test as a substitute.

### 3. Real identities, receipts and notification inventory

`withRuntime()` (`collaboration-recovery-runtime.ts:285`) registers four separate password actors and creates separate source/resource workspaces. The owner enrolls a real Ed25519-signed installation, binds a PAT-issued managed daemon identity and registers one runtime (`collaboration-host-runtime.ts:197`). The owner is creator/resource owner; handler and external participant have no source-workspace membership; outsider is never invited. Public issue creation, transfer, invitation, agent offer/adoption and resource consent precede execution (`createRuntimeIssue():368`).

`runtimeTransport():82` holds genuine requests in private memory. It forwards the original bytes and headers, observes the actual issued claim/permit, and changes only proof timing or drops successful outcome/proof ACKs. Credentials remain separate: business `mcb_`, termination `mct_`, daemon `mdt_`; evidence contains hashes. Claim/permit responses require original run identity, protocol 3, attempt 1, negotiated handoff/recording 1. This is an observation/replay transport, not a supplied authorization/proof.

The four tops plan **ten subcases, ten intentionally launched Provider executions and twelve original human inputs**. Preparation probes and run-local CLI subprocesses are additional process observations, not the ten-launch denominator. Every top creates its full NOT_RUN inventory before setup (`inventory():450`), so later failures cannot erase unstarted subcases.

Notifications execute failure, then explicit success, then Stop on the same issue (`exerciseRecoveryRuntimeNotifications():695`). Each has one input/run/group/attempt. With the real proof held, run proof remains unknown and no result notice exists. After its actual proof commits, exactly the creator/owner, handler and participating viewer receive one `ai_failed`, `ai_completed` or `ai_stopped` notification each; outsider gets none. The shared UI opens the actual `#run-<id>` anchor and run records. Original failed run/records/notice IDs must survive later success.

`recoverOriginal():602` stops only the current poller, retains the original directory/ledger, restarts a distinct process with the same runtime, compares original outcome/proof/launch hashes and receipts, and requires no new Claim/Permit/callback. Original input and Stop replay leave the complete audit rows unchanged. An actually recorded outcome receives only its original content-free ACK; absent outcome is explicitly not claimed. Changed original outcome conflicts; a fresh late event is refused.

### 4. Runtime withdrawal coverage

`establishProtectedReads():791` first proves 200 with the **original actually issued business token** for context, preparation, exact input file bytes, comment page/content, memory, live handoff, original snapshot replay, original memory base and each genuine source chunk. It validates the input/file/source IDs and hashes. The audit joins its capability hash to `collaboration_business_capability` before revocation.

`exerciseRecoveryRuntimeWithdrawal():865` creates two separate issues per platform:

1. External participant authors one live and one independent waiting input. The test suspends only its owned poller, removes the actual participant through the public operation API, requires waiting cancellation/no consuming input, and checks the old author/participant/account/input identities remain unchanged.
2. Handler authors a live input; creator/resource owner revokes the exact original grant through the public grant route while the owned Provider remains live.

Every previously authorized path repeats with that same business token and records actual refusal status/code without issue title/description. The authored matcher currently admits 403/404/409 rather than pinning one code for every reader; final review must assess the retained per-path codes, not just aggregate denial counts. It then resumes only its poller, completes the real supervisor proof and replays original receipts without reenrollment/Resume/reconsent. Removed human authors receive content-free comment receipts and lose the issue surface; retained handlers can still read the issue after resource-grant loss. No run-local Agent output upload is implemented here.

### 5. Gates, cleanup and coordinator-owned evidence

`openHostAcceptanceGate():127` locks the dedicated gate row, saves creation/execution/host flags, opens only missing flags and advances epoch, then restores original flags with another monotonic epoch only when required. `withRuntime()` releases proof barriers, closes the daemon, closes transport and restores the gate even after primary failure. It reports cleanup failures separately and preserves the original failure.

`startHostDaemon():239` copies only the explicit Provider/CLI into a private directory. `stopPreservingState()` exports its actual poller PID and retains state; `restart()` exports old/new PID. Final `close()` sends EOF (resuming a suspended POSIX poller first), waits up to 20 seconds, exports allowlisted ledger and removes the credential-bearing directory. Its fallback SIGKILL remains a failure, not normal stop. The final close summary does not enumerate all Provider/CLI PIDs; ledger redaction replaces native process identity with `process_observed`. Main must record its owned live daemon/Provider/run-local CLI process identities and lineage **before** cleanup, and independently prove no owned listener/process/connection remains. Raw credential-bearing ledger must not be published or copied as public evidence.

`withSessions():353` uses genuine Web cookies/CSRF or native bearer authentication, retains session/native lifecycle and asserts the Desktop fixture `daemonStarts=0`. Controlled poller/Provider are separate owned processes; installed Electron Main and commercial model behavior are outside this evidence.

The fixture's bounded read-only audit enumerates 21 real relation sets with actual session-to-issue joins. It covers commands, groups/runs/attempts, capability hashes, reservations, grants, participant/source fences, input consumption, events, operations, collaboration notices and uploaded input/reference rows. Ordinary `agent_task_queue`, `attachment`, `inbox_item` and `issue_subscriber` must stay empty. Ordinary subscription acceptance therefore belongs to a **different database/run slice**.

### 6. Minimal coordinator execution shape

1. Assemble a new coherent harness using the two sealed runtime files, current shared helpers/registration fixture and actual accepted server R05/client R05 package graphs. Do not mutate any prior frozen source, build, database or result. Bind every link and executable.
2. Run strict targeted TypeScript, targeted ESLint and exact four-top `--list` on that graph; preserve new failures separately. Compile the three explicit runtime executables and perform their readiness checks. Review actual source/config/binary identities before runtime entry.
3. Allocate a fresh empty full-migration/index database and closed API/browser configuration with pool budgeting. Verify database, deployment, actual listener ownership, production Web API/WS origin and matching renderer/preload. Client R05 build has API/WS bound to port 64940; either reuse that exact stopped/owned port with the fresh database, or rebuild for a new origin. Do not change only test-child URLs.
4. Run the full spec once with the pinned Node 22 / Playwright entry, `--workers=1 --retries=0 --max-failures=0`, JSON output and distinct test-results path. No old round contributes missing cases.
5. Before/after stopping only owned API/Web/renderer, retain source/config/build digests, actual gate snapshots, full original case inventories, relation/receipt audits, owned process closure and dedicated DB connection count. Independently review complete evidence before promoting any matrix subassertion.

The old private `helpers/run-command.py` still defaults to `source-r01` manifests and its `launch-services.py` hardcodes original r01 sources/ports/commit. Neither can be used unchanged as proof of this final runtime deployment. Main may reuse their patterns in a new bound owner-local recorder; the actual accepted-source, harness and runtime executable manifest must be included at both ends.

### 7. S4 dependency observation

The existing `11-13-pay-241-s4-external-integrations/research/integration-readiness{,-followup-20261114}.md` still records E01–E07 NOT_RUN and missing supplied I02–I09: real repository/CI and native build identities; PAY order/payment/fulfillment sandbox; supported duplicate/delay/consumer controls; nonproduction release/rollback targets; actual metric rules/samples; designated human reviewers. This inspection did not inventory credentials or contact external accounts/targets, so it does not claim no such resources exist elsewhere. Multica Cloud Stripe billing remains a different business boundary. S5's final-candidate 100-user acceptance cannot consume these static or controlled-runtime results as real S4 evidence.

## External references and related specs

- No external documentation or service was contacted. Current repository module, retained frontend build report and actual source contracts are the version references.
- `.trellis/spec/server/collaboration-host-execution.md`: native host mode, per-launch lifecycle, distinct business/termination credentials and truthful cleanup scope.
- `.trellis/spec/server/collaboration-runtime-recovery.md`: retained original receipt/launch recovery, no fresh dispatch from uncertainty.
- `.trellis/spec/server/database-test-isolation.md` and `verification-environment.md`: fresh exact database/config/source admission and before/after audit.
- `.trellis/spec/web/frontend/e2e-run-environment.md` and `collaboration-recovery-qa.md`: production Web, actual native/WS scope, ordinary subscriber versus participant delivery, registration limiter preservation.
- Task research: `recovery-runtime-intake-20261118.md`, `recovery-runtime-implementation-20261118.md`, `block-ui-build-r05-20261118.md`, `block-package-r05-independent-check-20261118.md`, `block-runtime-r08-independent-check-20261118.md`.

## Caveats / Not Found

- This research ran no DB/API/browser/Provider/Go test/build body or process control; no config or production/e2e source was changed. No runtime PASS is claimed.
- The two owner seals and several exact helper hashes were checked, not a fresh complete shared-worktree or compiled-artifact acceptance.
- No definite scenario/product defect was established. Concrete preparation blockers are the absent prospective combined source, obsolete helper graph if copied naively, missing retained runtime configuration/binaries and origin/ownership binding. Actual execution can still reveal defects.
- This lane did not explain the old valid pagination GET 503; preserve that original unknown root cause and accepted later regression separately.
- Actual outcome completeness, denial codes, full PID/ledger closure, migration/index/connection checks and final matrix updates remain coordinator plus independent-check responsibilities.
